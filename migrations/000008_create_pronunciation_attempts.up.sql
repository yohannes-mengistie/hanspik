CREATE TABLE pronunciation_attempts (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    word_id            UUID NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    audio_path         TEXT NOT NULL,
    accuracy_score     DOUBLE PRECISION NOT NULL,
    fluency_score      DOUBLE PRECISION NOT NULL,
    completeness_score DOUBLE PRECISION NOT NULL,
    tone_score         DOUBLE PRECISION,
    overall_score      DOUBLE PRECISION NOT NULL,
    details            JSONB NOT NULL DEFAULT '{}'::jsonb,
    provider           TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pron_student_time ON pronunciation_attempts(student_id, created_at DESC);
CREATE INDEX idx_pron_word ON pronunciation_attempts(word_id);
