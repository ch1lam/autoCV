CREATE TABLE match_analyses (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    jd_id TEXT NOT NULL REFERENCES job_descriptions(id) ON DELETE CASCADE,
    input_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('succeeded', 'failed')),
    error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (profile_id, jd_id)
);

CREATE INDEX match_analyses_updated_at_idx
    ON match_analyses(updated_at DESC);

CREATE TABLE match_requirements (
    analysis_id TEXT NOT NULL REFERENCES match_analyses(id) ON DELETE CASCADE,
    id TEXT NOT NULL,
    category TEXT NOT NULL CHECK (
        category IN ('required', 'responsibility', 'level', 'domain', 'preferred')
    ),
    text TEXT NOT NULL,
    importance INTEGER NOT NULL CHECK (importance BETWEEN 1 AND 5),
    hard_constraint INTEGER NOT NULL CHECK (hard_constraint IN (0, 1)),
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    PRIMARY KEY (analysis_id, id)
);

CREATE TABLE requirement_matches (
    analysis_id TEXT NOT NULL,
    requirement_id TEXT NOT NULL,
    strength TEXT NOT NULL CHECK (
        strength IN ('strong', 'partial', 'missing', 'unknown')
    ),
    explanation TEXT NOT NULL,
    clarification_needed INTEGER NOT NULL CHECK (
        clarification_needed IN (0, 1)
    ),
    PRIMARY KEY (analysis_id, requirement_id),
    FOREIGN KEY (analysis_id, requirement_id)
        REFERENCES match_requirements(analysis_id, id)
        ON DELETE CASCADE
);

CREATE TABLE match_evidence (
    analysis_id TEXT NOT NULL,
    requirement_id TEXT NOT NULL,
    evidence_id TEXT NOT NULL REFERENCES evidence(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    PRIMARY KEY (analysis_id, requirement_id, evidence_id),
    FOREIGN KEY (analysis_id, requirement_id)
        REFERENCES requirement_matches(analysis_id, requirement_id)
        ON DELETE CASCADE
);

CREATE INDEX match_evidence_evidence_id_idx
    ON match_evidence(evidence_id);
