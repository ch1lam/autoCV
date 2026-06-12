ALTER TABLE artifacts
    ADD COLUMN resume_id TEXT REFERENCES resumes(id) ON DELETE CASCADE;

CREATE INDEX artifacts_resume_id_idx ON artifacts(resume_id);

CREATE INDEX artifacts_latest_idx
    ON artifacts(run_id, kind, created_at DESC);
