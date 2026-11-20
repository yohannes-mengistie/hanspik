package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yohannes/chines-backend/internal/config"
	"github.com/yohannes/chines-backend/internal/platform/database"
	"github.com/yohannes/chines-backend/internal/user"
	"github.com/yohannes/chines-backend/migrations"
)

// Catalog is the on-disk HSK 1 content. It is the single source of truth for
// the seeded class; the mobile app ships the same list.
type Catalog struct {
	Class struct {
		Name string `json:"name"`
	} `json:"class"`
	Lessons []struct {
		Slug    string `json:"slug"`
		Index   int    `json:"index"`
		Title   string `json:"title"`
		TitleZh string `json:"title_zh"`
		Summary string `json:"summary"`
		Seal    string `json:"seal"`
		Words   []Word `json:"words"`
	} `json:"lessons"`
}

type Word struct {
	Slug            string `json:"slug"`
	Hanzi           string `json:"hanzi"`
	Pinyin          string `json:"pinyin"`
	Meaning         string `json:"meaning"`
	ExampleSentence string `json:"example_sentence"`
	ExamplePinyin   string `json:"example_pinyin"`
	ExampleMeaning  string `json:"example_meaning"`
}

func main() {
	catalogPath := flag.String("catalog", "content/hsk1.json", "path to the catalog JSON")
	flag.Parse()

	raw, err := os.ReadFile(*catalogPath)
	if err != nil {
		log.Fatalf("read catalog: %v", err)
	}
	var cat Catalog
	if err := json.Unmarshal(raw, &cat); err != nil {
		log.Fatalf("parse catalog: %v", err)
	}

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := migrations.Up(ctx, db); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	email := env("SEED_TEACHER_EMAIL", "teacher@chines.local")
	password := env("SEED_TEACHER_PASSWORD", "teach12345")
	name := env("SEED_TEACHER_NAME", "Seed Teacher")

	users := user.NewRepository(db)
	teacher, err := users.GetByEmail(ctx, email)
	if err == user.ErrNotFound {
		hash, herr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if herr != nil {
			log.Fatalf("hash password: %v", herr)
		}
		teacher, err = users.Create(ctx, name, email, string(hash), user.RoleTeacher)
	}
	if err != nil {
		log.Fatalf("teacher: %v", err)
	}

	// Class: one "HSK 1" per seed teacher.
	var classID string
	err = db.QueryRow(ctx,
		`SELECT id::text FROM classes WHERE teacher_id = $1 AND name = $2 LIMIT 1`,
		teacher.ID, cat.Class.Name).Scan(&classID)
	if err != nil {
		if err := db.QueryRow(ctx,
			`INSERT INTO classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`,
			cat.Class.Name, teacher.ID).Scan(&classID); err != nil {
			log.Fatalf("create class: %v", err)
		}
	}

	wordCount := 0
	for _, lesson := range cat.Lessons {
		var lessonID string
		err := db.QueryRow(ctx,
			`SELECT id::text FROM lessons WHERE class_id = $1 AND title = $2 LIMIT 1`,
			classID, lesson.Title).Scan(&lessonID)
		if err != nil {
			date := time.Now().Format("2006-01-02")
			if err := db.QueryRow(ctx,
				`INSERT INTO lessons (class_id, title, lesson_date) VALUES ($1, $2, $3) RETURNING id::text`,
				classID, lesson.Title, date).Scan(&lessonID); err != nil {
				log.Fatalf("create lesson %q: %v", lesson.Title, err)
			}
		}

		for i, w := range lesson.Words {
			wordCount++
			// Upsert by (lesson, hanzi): keeps hand-edited content in sync.
			_, err := db.Exec(ctx, `
				INSERT INTO words (lesson_id, hanzi, pinyin, meaning, example_sentence, position)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT DO NOTHING`,
				lessonID, w.Hanzi, w.Pinyin, w.Meaning, w.ExampleSentence, i+1)
			if err != nil {
				log.Fatalf("create word %q: %v", w.Hanzi, err)
			}
			if _, err := db.Exec(ctx, `
				UPDATE words SET pinyin = $3, meaning = $4, example_sentence = $5, position = $6
				WHERE lesson_id = $1 AND hanzi = $2`,
				lessonID, w.Hanzi, w.Pinyin, w.Meaning, w.ExampleSentence, i+1); err != nil {
				log.Fatalf("update word %q: %v", w.Hanzi, err)
			}
		}
	}

	log.Printf("seed ok: teacher=%s class=%s lessons=%d words=%d",
		email, cat.Class.Name, len(cat.Lessons), wordCount)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
