package pronunciation

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestFakeScorerIsDeterministicAndInRange(t *testing.T) {
	scorer := NewFakeScorer()
	audio := []byte("RIFF....WAVEfmt ")

	first, err := scorer.Score(context.Background(), audio, "你好", "zh-CN")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	second, err := scorer.Score(context.Background(), audio, "你好", "zh-CN")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if first.Accuracy != second.Accuracy || first.Fluency != second.Fluency ||
		first.Completeness != second.Completeness || first.Overall != second.Overall ||
		string(first.Details) != string(second.Details) {
		t.Fatalf("expected deterministic scores")
	}

	scores := map[string]float64{
		"accuracy":     first.Accuracy,
		"fluency":      first.Fluency,
		"completeness": first.Completeness,
		"tone":         *first.Tone,
		"overall":      first.Overall,
	}
	for name, value := range scores {
		if value < 0 || value > 100 {
			t.Errorf("%s = %.1f, want 0..100", name, value)
		}
	}
	if first.Provider != "fake" {
		t.Errorf("provider = %q, want fake", first.Provider)
	}

	var details struct {
		Syllables []SyllableDetail `json:"syllables"`
	}
	if err := json.Unmarshal(first.Details, &details); err != nil {
		t.Fatalf("details: %v", err)
	}
	if len(details.Syllables) != 2 {
		t.Fatalf("syllables = %d, want 2", len(details.Syllables))
	}
	if details.Syllables[0].Syllable != "你" || details.Syllables[1].Syllable != "好" {
		t.Errorf("unexpected syllables: %+v", details.Syllables)
	}
}

func TestFakeScorerRejectsEmptyAudio(t *testing.T) {
	if _, err := NewFakeScorer().Score(context.Background(), nil, "你好", "zh-CN"); err == nil {
		t.Fatal("expected an error for empty audio")
	}
}

func TestNewScorerSelection(t *testing.T) {
	if _, err := NewScorer("fake", "", ""); err != nil {
		t.Fatalf("fake: %v", err)
	}
	if _, err := NewScorer("", "", ""); err != nil {
		t.Fatalf("default: %v", err)
	}
	if _, err := NewScorer("azure", "", ""); err == nil {
		t.Fatal("expected an error when Azure credentials are missing")
	}
	if _, err := NewScorer("nope", "", ""); err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
	if _, err := NewScorer("azure", "key", "westus"); err != nil {
		t.Fatalf("azure: %v", err)
	}
}

func TestLimiterWindow(t *testing.T) {
	now := time.Now()
	limiter := NewLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	if !limiter.Allow("s1") || !limiter.Allow("s1") {
		t.Fatal("first two attempts should pass")
	}
	if limiter.Allow("s1") {
		t.Fatal("third attempt should be limited")
	}
	if !limiter.Allow("s2") {
		t.Fatal("another student must have their own window")
	}

	now = now.Add(61 * time.Second)
	if !limiter.Allow("s1") {
		t.Fatal("window should have expired")
	}
}

func TestAudioDetectionMap(t *testing.T) {
	cases := map[string]string{
		"RIFF....WAVEfmt ":    ".wav",
		"ID3\x04\x00\x00\x00": ".mp3",
		"OggS\x00\x02\x00":    ".ogg",
	}
	for head, want := range cases {
		got, ok := audioExt[http.DetectContentType([]byte(head))]
		if !ok || got != want {
			t.Errorf("detect %q = %q (ok=%v), want %q", head, got, ok, want)
		}
	}
}
