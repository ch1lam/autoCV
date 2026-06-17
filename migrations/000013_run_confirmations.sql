CREATE TABLE run_confirmations (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES resume_runs(id) ON DELETE CASCADE,
    clarification_question_id TEXT NOT NULL
        REFERENCES clarification_questions(id) ON DELETE CASCADE,
    requirement_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (run_id, clarification_question_id)
);

CREATE INDEX run_confirmations_run_id_idx
    ON run_confirmations(run_id);

CREATE INDEX run_confirmations_requirement_id_idx
    ON run_confirmations(requirement_id);
