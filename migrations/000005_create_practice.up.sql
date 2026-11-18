CREATE TABLE word_progress (
    student_id       UUID             NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    word_id          UUID             NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    ease_factor      DOUBLE PRECISION NOT NULL DEFAULT 2.5 CHECK (ease_factor >= 1.3),
    interval_days    INT              NOT NULL DEFAULT 0,
    repetitions      INT              NOT NULL DEFAULT 0,
    next_review_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),
    last_reviewed_at TIMESTAMPTZ,
    PRIMARY KEY (student_id, word_id)
);

CREATE INDEX idx_word_progress_due ON word_progress(student_id, next_review_at);

CREATE TABLE practice_attempts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    word_id    UUID        NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    type       TEXT        NOT NULL CHECK (type IN ('flashcard', 'choice', 'typing')),
    quality    SMALLINT    NOT NULL CHECK (quality BETWEEN 0 AND 5),
    is_correct BOOLEAN     NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_attempts_student_time ON practice_attempts(student_id, created_at);