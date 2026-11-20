package pronunciation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"time"
)

// NewScorer selects the provider from configuration. Unknown values are a
// startup error instead of a silent fallback, so a typo cannot disable scoring.
func NewScorer(provider, key, region string) (Scorer, error) {
	switch provider {
	case "", "fake":
		return NewFakeScorer(), nil
	case "azure":
		if key == "" || region == "" {
			return nil, errors.New("SCORER=azure requires AZURE_SPEECH_KEY and AZURE_SPEECH_REGION")
		}
		return NewAzureScorer(key, region, 20*time.Second), nil
	default:
		return nil, fmt.Errorf("unknown SCORER %q (use fake or azure)", provider)
	}
}

// FakeScorer is deterministic: the same audio and text always produce the same
// scores. It lets the whole flow run locally and in tests without credentials.
type FakeScorer struct{}

func NewFakeScorer() *FakeScorer { return &FakeScorer{} }

func (f *FakeScorer) Score(_ context.Context, audio []byte, referenceText, _ string) (*Result, error) {
	if len(audio) == 0 {
		return nil, ErrUnsupportedAudio
	}
	sum := sha256.Sum256(append([]byte(referenceText), audio...))
	h := fnv.New32a()
	_, _ = h.Write(sum[:])
	base := h.Sum32()

	clamp := func(n uint32, low, high float64) float64 {
		span := uint32(high - low)
		return low + float64(n%span)
	}
	accuracy := clamp(base>>1, 60, 99)
	fluency := clamp(base>>3, 60, 99)
	completeness := clamp(base>>5, 60, 100)
	tone := clamp(base>>7, 60, 99)
	overall := (accuracy*0.5 + fluency*0.2 + completeness*0.15 + tone*0.15)

	var syllables []SyllableDetail
	for _, r := range referenceText {
		if len(syllables) >= 8 {
			break
		}
		if r == ' ' {
			continue
		}
		value := clamp(base+uint32(r), 55, 99)
		t := clamp(base+(uint32(r)>>2), 55, 99)
		syllables = append(syllables, SyllableDetail{
			Syllable: string(r),
			Accuracy: value,
			Tone:     &t,
		})
	}
	details, err := json.Marshal(map[string]any{"syllables": syllables})
	if err != nil {
		return nil, err
	}
	return &Result{
		Accuracy:     accuracy,
		Fluency:      fluency,
		Completeness: completeness,
		Tone:         &tone,
		Overall:      overall,
		Provider:     "fake",
		Details:      details,
	}, nil
}

// AzureScorer calls Azure Speech Pronunciation Assessment for zh-CN.
// It accepts 16 kHz mono PCM WAV, which is what the mobile app records.
type AzureScorer struct {
	key    string
	region string
	client *http.Client
}

func NewAzureScorer(key, region string, timeout time.Duration) *AzureScorer {
	return &AzureScorer{
		key:    key,
		region: region,
		client: &http.Client{Timeout: timeout},
	}
}

type azureResponse struct {
	RecognitionStatus string `json:"RecognitionStatus"`
	NBest             []struct {
		AccuracyScore     float64     `json:"AccuracyScore"`
		FluencyScore      float64     `json:"FluencyScore"`
		CompletenessScore float64     `json:"CompletenessScore"`
		PronScore         float64     `json:"PronScore"`
		ToneScore         *float64    `json:"ToneScore"`
		Words             []azureWord `json:"Words"`
	} `json:"NBest"`
}

type azureWord struct {
	Word          string   `json:"Word"`
	AccuracyScore float64  `json:"AccuracyScore"`
	ToneScore     *float64 `json:"ToneScore"`
	ErrorType     string   `json:"ErrorType"`
	Syllables     []struct {
		Syllable      string   `json:"Syllable"`
		AccuracyScore float64  `json:"AccuracyScore"`
		ToneScore     *float64 `json:"ToneScore"`
	} `json:"Syllables"`
}

func (a *AzureScorer) Score(ctx context.Context, audio []byte, referenceText, lang string) (*Result, error) {
	if !isWAV(audio) {
		return nil, fmt.Errorf("%w: Azure needs 16 kHz mono PCM WAV", ErrUnsupportedAudio)
	}
	if lang == "" {
		lang = "zh-CN"
	}

	params := map[string]any{
		"ReferenceText":           referenceText,
		"GradingSystem":           "HundredMark",
		"Granularity":             "Word",
		"Dimension":               "Comprehensive",
		"EnableMiscue":            true,
		"EnableProsodyAssessment": false,
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("https://%s.stt.speech.microsoft.com/speech/recognition/conversation/cognitiveservices/v1",
		url.PathEscape(a.region))
	q := url.Values{}
	q.Set("language", lang)
	q.Set("format", "detailed")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+q.Encode(), bytes.NewReader(audio))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", a.key)
	req.Header.Set("Content-Type", "audio/wav; codecs=audio/pcm; samplerate=16000")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Pronunciation-Assessment", base64.StdEncoding.EncodeToString(raw))

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	if resp.StatusCode != http.StatusOK {
		// The body can contain provider diagnostics; keep it in the log only.
		return nil, fmt.Errorf("%w: status %d", ErrProvider, resp.StatusCode)
	}

	var out azureResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%w: bad response", ErrProvider)
	}
	if out.RecognitionStatus != "Success" || len(out.NBest) == 0 {
		return nil, fmt.Errorf("%w: recognition status %q", ErrProvider, out.RecognitionStatus)
	}

	best := out.NBest[0]
	syllables := make([]SyllableDetail, 0, len(best.Words))
	toneSum, toneCount := 0.0, 0
	for _, w := range best.Words {
		if w.Syllables != nil {
			for _, s := range w.Syllables {
				text := s.Syllable
				if text == "" {
					text = w.Word
				}
				syllables = append(syllables, SyllableDetail{
					Syllable:  text,
					Accuracy:  s.AccuracyScore,
					Tone:      s.ToneScore,
					ErrorType: w.ErrorType,
				})
				if s.ToneScore != nil {
					toneSum += *s.ToneScore
					toneCount++
				}
			}
			continue
		}
		syllables = append(syllables, SyllableDetail{
			Syllable:  w.Word,
			Accuracy:  w.AccuracyScore,
			Tone:      w.ToneScore,
			ErrorType: w.ErrorType,
		})
		if w.ToneScore != nil {
			toneSum += *w.ToneScore
			toneCount++
		}
	}

	var tone *float64
	if toneCount > 0 {
		avg := toneSum / float64(toneCount)
		tone = &avg
	}
	details, err := json.Marshal(map[string]any{"syllables": syllables})
	if err != nil {
		return nil, err
	}
	return &Result{
		Accuracy:     best.AccuracyScore,
		Fluency:      best.FluencyScore,
		Completeness: best.CompletenessScore,
		Tone:         tone,
		Overall:      best.PronScore,
		Provider:     "azure",
		Details:      details,
	}, nil
}

func isWAV(audio []byte) bool {
	return len(audio) >= 12 && bytes.Equal(audio[0:4], []byte("RIFF")) && bytes.Equal(audio[8:12], []byte("WAVE"))
}
