package lesson

import "time"

type Word struct {
	ID              string  `json:"id"`
	LessonID        string  `json:"lesson_id"`
	Hanzi           string  `json:"hanzi"`
	Pinyin          string  `json:"pinyin"`
	Meaning         string  `json:"meaning"`
	ExampleSentence string  `json:"example_sentence"`
	AudioURL        *string `json:"audio_url"` // pointer, so NULL becomes JSON null
	Position        int     `json:"position"`
}

type Lesson struct {
	ID         string    `json:"id"`
	ClassID    string    `json:"class_id"`
	Title      string    `json:"title"`
	LessonDate string    `json:"lesson_date"` // "2026-11-14"
	CreatedAt  time.Time `json:"created_at"`
	Words      []Word    `json:"words,omitempty"`
}
