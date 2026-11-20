package practice

import (
	"math"
	"testing"
	"time"
)

func TestNextSequence(t *testing.T) {
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	s := State{EaseFactor: defaultEase}

	steps := []struct {
		quality  int
		interval int
		reps     int
		ease     float64
	}{
		{4, 1, 1, 2.5},  // first success: 1 day
		{4, 6, 2, 2.5},  // second success: 6 days
		{4, 15, 3, 2.5}, // then interval * ease = 6 * 2.5
		{5, 38, 4, 2.6}, // "easy" raises the ease factor (15 * 2.5 = 37.5 rounds to 38)
		{1, 1, 0, 2.6},  // forgotten: back to 1 day, ease unchanged
	}

	for i, st := range steps {
		s = Next(s, st.quality, now)
		if s.IntervalDays != st.interval || s.Repetitions != st.reps ||
			math.Abs(s.EaseFactor-st.ease) > 0.0001 {
			t.Fatalf("step %d: got interval=%d reps=%d ease=%.2f, want interval=%d reps=%d ease=%.2f",
				i+1, s.IntervalDays, s.Repetitions, s.EaseFactor, st.interval, st.reps, st.ease)
		}
	}
}

func TestEaseNeverBelowMinimum(t *testing.T) {
	s := State{EaseFactor: 1.4, Repetitions: 2, IntervalDays: 6}
	for i := 0; i < 10; i++ {
		s = Next(s, 3, time.Now()) // "hard" lowers the ease each time
	}
	if s.EaseFactor < minEase {
		t.Fatalf("ease fell below %.1f: %.2f", minEase, s.EaseFactor)
	}
}

func TestNextReviewDate(t *testing.T) {
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	s := Next(State{EaseFactor: defaultEase}, 4, now)
	want := now.Add(24 * time.Hour)
	if !s.NextReviewAt.Equal(want) {
		t.Fatalf("got %v, want %v", s.NextReviewAt, want)
	}
}
