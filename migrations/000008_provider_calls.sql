CREATE TABLE provider_calls (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    task TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (
        status IN ('succeeded', 'failed', 'cancelled')
    ),
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0),
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    total_tokens INTEGER NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
    schema_repaired INTEGER NOT NULL DEFAULT 0 CHECK (
        schema_repaired IN (0, 1)
    ),
    error_kind TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX provider_calls_created_at_idx
    ON provider_calls(created_at DESC);
