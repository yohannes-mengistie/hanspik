CREATE TABLE lessons (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id    UUID        NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    title       TEXT        NOT NULL,
    lesson_date DATE        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_lessons_class_date ON lessons(class_id, lesson_date);

CREATE TABLE words (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id        UUID        NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    hanzi            TEXT        NOT NULL,
    pinyin           TEXT        NOT NULL,
    meaning          TEXT        NOT NULL,
    example_sentence TEXT        NOT NULL DEFAULT '',
    audio_url        TEXT,                      -- filled in a later step (audio upload)
    position         INT         NOT NULL,      -- order inside the lesson
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_words_lesson ON words(lesson_id, position);