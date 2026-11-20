package practice

import (
	"math"
	"time"
)

const (
	defaultEase = 2.5
	minEase     = 1.3
)

// State is what we remember about one student and one word.
type State struct {
	EaseFactor   float64
	IntervalDays int
	Repetitions  int
	NextReviewAt time.Time
}

// Next applies one answer (quality 0-5) and returns the new state.
func Next(cur State, quality int, now time.Time) State {
	next := cur

	if quality < 3 {
		// Forgotten: start again. The ease factor is not changed.
		next.Repetitions = 0
		next.IntervalDays = 1
	} else {
		switch cur.Repetitions {
		case 0:
			next.IntervalDays = 1
		case 1:
			next.IntervalDays = 6
		default:
			next.IntervalDays = int(math.Round(float64(cur.IntervalDays) * cur.EaseFactor))
		}
		next.Repetitions = cur.Repetitions + 1

		q := float64(5 - quality)
		next.EaseFactor = cur.EaseFactor + (0.1 - q*(0.08+q*0.02))
		if next.EaseFactor < minEase {
			next.EaseFactor = minEase
		}
	}

	next.NextReviewAt = now.Add(time.Duration(next.IntervalDays) * 24 * time.Hour)
	return next
}
