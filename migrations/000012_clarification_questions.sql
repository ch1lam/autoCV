CREATE TABLE clarification_questions (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES resume_runs(id) ON DELETE CASCADE,
    requirement_id TEXT NOT NULL DEFAULT '',
    round INTEGER NOT NULL CHECK (round BETWEEN 1 AND 2),
    ordinal INTEGER NOT NULL CHECK (ordinal BETWEEN 0 AND 4),
    question TEXT NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL CHECK (
        status IN ('pending', 'answered', 'skipped')
    ),
    answer TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (run_id, round, ordinal)
);

CREATE INDEX clarification_questions_run_id_idx
    ON clarification_questions(run_id);

CREATE INDEX clarification_questions_requirement_id_idx
    ON clarification_questions(requirement_id);
