package pronunciation

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const MaxAudioBytes = 2 << 20 // 2 MB, same limit as word audio

var (
	ErrRateLimited      = errors.New("too many pronunciation attempts")
	ErrUnsupportedAudio = errors.New("unsupported audio format")
	ErrProvider         = errors.New("pronunciation provider failed")
)

// Result is what a Scorer returns for one recording.
type Result struct {
	Accuracy     float64
	Fluency      float64
	Completeness float64
	Tone         *float64
	Overall      float64
	Provider     string
	Details      json.RawMessage
}

// Scorer turns audio into scores. FakeScorer is used in tests/local dev;
// AzureScorer calls Azure Speech Pronunciation Assessment.
type Scorer interface {
	Score(ctx context.Context, audio []byte, referenceText, lang string) (*Result, error)
}

type Attempt struct {
	ID           string          `json:"id"`
	StudentID    string          `json:"student_id"`
	WordID       string          `json:"word_id"`
	AudioPath    string          `json:"-"`
	Accuracy     float64         `json:"accuracy_score"`
	Fluency      float64         `json:"fluency_score"`
	Completeness float64         `json:"completeness_score"`
	Tone         *float64        `json:"tone_score"`
	Overall      float64         `json:"overall_score"`
	Details      json.RawMessage `json:"details"`
	Provider     string          `json:"provider"`
	CreatedAt    time.Time       `json:"created_at"`
}

// SyllableDetail is the stable shape written into details.syllables so the
// mobile app can highlight the syllable and tone that went wrong.
type SyllableDetail struct {
	Syllable  string   `json:"syllable"`
	Accuracy  float64  `json:"accuracy_score"`
	Tone      *float64 `json:"tone_score,omitempty"`
	ErrorType string   `json:"error_type,omitempty"`
}
