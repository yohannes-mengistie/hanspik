package teacher

import (
	"testing"
	"time"
)

func TestStreakFromDays(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	day := func(offset int) string {
		return now.AddDate(0, 0, offset).Format("2006-01-02")
	}
	set := func(offsets ...int) map[string]bool {
		m := map[string]bool{}
		for _, o := range offsets {
			m[day(o)] = true
		}
		return m
	}

	cases := []struct {
		name string
		days map[string]bool
		want int
	}{
		{"empty", set(), 0},
		{"today only", set(0), 1},
		{"today and yesterday", set(0, -1), 2},
		{"gap ends streak", set(0, -2, -3), 1},
		{"yesterday counts as current", set(-1, -2), 2},
		{"stale is zero", set(-3, -4), 0},
		{"long run", set(0, -1, -2, -3, -4, -5), 6},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := streakFromDays(tc.days, now); got != tc.want {
				t.Fatalf("streakFromDays() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPercent(t *testing.T) {
	cases := []struct {
		correct, attempts int64
		want              float64
	}{
		{0, 0, 0},
		{0, 4, 0},
		{3, 4, 75},
		{5, 5, 100},
	}
	for _, tc := range cases {
		if got := percent(tc.correct, tc.attempts); got != tc.want {
			t.Fatalf("percent(%d,%d) = %v, want %v", tc.correct, tc.attempts, got, tc.want)
		}
	}
}
