CREATE TABLE profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    default_language TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE source_documents (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    original_name TEXT NOT NULL,
    managed_path TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    parse_status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX source_documents_profile_id_idx
    ON source_documents(profile_id);

CREATE TABLE job_descriptions (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    company TEXT NOT NULL DEFAULT '',
    raw_text TEXT NOT NULL,
    language TEXT NOT NULL,
    analysis_json TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE resume_runs (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    jd_id TEXT NOT NULL REFERENCES job_descriptions(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    stage TEXT NOT NULL,
    packaging_level REAL NOT NULL CHECK (
        packaging_level >= 0 AND packaging_level <= 1
    ),
    language TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX resume_runs_profile_id_idx ON resume_runs(profile_id);
CREATE INDEX resume_runs_jd_id_idx ON resume_runs(jd_id);

CREATE TABLE stage_results (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES resume_runs(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    status TEXT NOT NULL,
    result_json TEXT,
    error_json TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (run_id, stage, input_hash)
);

CREATE INDEX stage_results_run_id_idx ON stage_results(run_id);

CREATE TABLE resumes (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES resume_runs(id) ON DELETE CASCADE,
    version INTEGER NOT NULL CHECK (version > 0),
    structure_json TEXT NOT NULL,
    markdown TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (run_id, version)
);

CREATE INDEX resumes_run_id_idx ON resumes(run_id);

CREATE TABLE artifacts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES resume_runs(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    path TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX artifacts_run_id_idx ON artifacts(run_id);

CREATE TABLE provider_configs (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    secret_ref TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX provider_configs_provider_idx
    ON provider_configs(provider);
