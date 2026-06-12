ALTER TABLE resumes
    ADD COLUMN input_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE resume_blocks (
    resume_id TEXT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (
        kind IN (
            'summary',
            'experience',
            'project',
            'skill',
            'education',
            'certification'
        )
    ),
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    content TEXT NOT NULL,
    locked INTEGER NOT NULL DEFAULT 0 CHECK (locked IN (0, 1)),
    grounding_level TEXT NOT NULL CHECK (
        grounding_level IN ('source', 'derived', 'user_confirmed')
    ),
    optimization TEXT NOT NULL,
    PRIMARY KEY (resume_id, id),
    UNIQUE (resume_id, ordinal)
);

CREATE TABLE block_sources (
    resume_id TEXT NOT NULL,
    block_id TEXT NOT NULL,
    evidence_id TEXT NOT NULL REFERENCES evidence(id) ON DELETE RESTRICT,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    relation TEXT NOT NULL DEFAULT 'supports',
    risk_level TEXT NOT NULL DEFAULT 'low',
    PRIMARY KEY (resume_id, block_id, evidence_id),
    FOREIGN KEY (resume_id, block_id)
        REFERENCES resume_blocks(resume_id, id)
        ON DELETE CASCADE
);

CREATE INDEX block_sources_evidence_id_idx
    ON block_sources(evidence_id);
