package teacher

import "time"

type Overview struct {
	Classes          int `json:"classes"`
	Students         int `json:"students"`
	ActiveStudents7d int `json:"active_students_7d"`
}

type StudentProgress struct {
	StudentID                 string     `json:"student_id"`
	Name                      string     `json:"name"`
	Email                     string     `json:"email"`
	WordsPracticed            int        `json:"words_practiced"`
	Accuracy                  float64    `json:"accuracy"`
	DueCount                  int        `json:"due_count"`
	LastActive                *time.Time `json:"last_active"`
	Streak                    int        `json:"streak"`
	AveragePronunciationScore *float64   `json:"average_pronunciation_score"`
}

type ClassProgress struct {
	ClassID   string            `json:"class_id"`
	ClassName string            `json:"class_name"`
	Students  []StudentProgress `json:"students"`
}

type LessonAccuracy struct {
	LessonID string  `json:"lesson_id"`
	Title    string  `json:"title"`
	Attempts int     `json:"attempts"`
	Correct  int     `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

type WordStat struct {
	WordID   string  `json:"word_id"`
	Hanzi    string  `json:"hanzi"`
	Pinyin   string  `json:"pinyin"`
	Meaning  string  `json:"meaning"`
	Attempts int     `json:"attempts"`
	Correct  int     `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

type Attempt struct {
	WordID    string    `json:"word_id"`
	Hanzi     string    `json:"hanzi"`
	Type      string    `json:"type"`
	Quality   int       `json:"quality"`
	IsCorrect bool      `json:"is_correct"`
	CreatedAt time.Time `json:"created_at"`
}

type StudentDetail struct {
	StudentID      string           `json:"student_id"`
	Name           string           `json:"name"`
	Email          string           `json:"email"`
	Lessons        []LessonAccuracy `json:"lessons"`
	HardestWords   []WordStat       `json:"hardest_words"`
	RecentAttempts []Attempt        `json:"recent_attempts"`
}

// streakFromDays counts consecutive days ending today (or yesterday, so a
// student is not marked "lost" before they have studied today). days holds
// "YYYY-MM-DD" keys in UTC.
func streakFromDays(days map[string]bool, now time.Time) int {
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if !days[d.Format("2006-01-02")] {
		d = d.AddDate(0, 0, -1)
		if !days[d.Format("2006-01-02")] {
			return 0
		}
	}
	n := 0
	for days[d.Format("2006-01-02")] {
		n++
		d = d.AddDate(0, 0, -1)
	}
	return n
}

func percent(correct, attempts int64) float64 {
	if attempts == 0 {
		return 0
	}
	return float64(correct) / float64(attempts) * 100
}
