CREATE UNIQUE INDEX source_documents_profile_hash_idx
    ON source_documents(profile_id, content_hash);

CREATE TABLE source_chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES source_documents(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    text TEXT NOT NULL,
    locator_json TEXT NOT NULL,
    UNIQUE (document_id, ordinal)
);

CREATE INDEX source_chunks_document_id_idx ON source_chunks(document_id);

CREATE TABLE evidence (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    user_verified INTEGER NOT NULL DEFAULT 0 CHECK (user_verified IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX evidence_profile_id_idx ON evidence(profile_id);

CREATE TABLE evidence_sources (
    evidence_id TEXT NOT NULL REFERENCES evidence(id) ON DELETE CASCADE,
    chunk_id TEXT NOT NULL REFERENCES source_chunks(id) ON DELETE CASCADE,
    quote_start INTEGER NOT NULL DEFAULT 0 CHECK (quote_start >= 0),
    quote_end INTEGER NOT NULL DEFAULT 0 CHECK (quote_end >= quote_start),
    PRIMARY KEY (evidence_id, chunk_id)
);

CREATE INDEX evidence_sources_chunk_id_idx ON evidence_sources(chunk_id);
