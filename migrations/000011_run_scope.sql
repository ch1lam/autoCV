CREATE TABLE run_scopes (
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    jd_id TEXT NOT NULL REFERENCES job_descriptions(id) ON DELETE CASCADE,
    mode TEXT NOT NULL CHECK (mode IN ('all', 'selected')),
    updated_at TEXT NOT NULL,
    PRIMARY KEY (profile_id, jd_id)
);

CREATE TABLE run_scope_documents (
    profile_id TEXT NOT NULL,
    jd_id TEXT NOT NULL,
    document_id TEXT NOT NULL REFERENCES source_documents(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    PRIMARY KEY (profile_id, jd_id, document_id),
    UNIQUE (profile_id, jd_id, ordinal),
    FOREIGN KEY (profile_id, jd_id)
        REFERENCES run_scopes(profile_id, jd_id)
        ON DELETE CASCADE
);

CREATE INDEX run_scope_documents_document_id_idx
    ON run_scope_documents(document_id);
