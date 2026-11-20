CREATE INDEX IF NOT EXISTS idx_attempts_word_id ON practice_attempts(word_id);
CREATE INDEX IF NOT EXISTS idx_word_progress_student_reviewed
    ON word_progress(student_id, last_reviewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_lessons_class_id ON lessons(class_id);
