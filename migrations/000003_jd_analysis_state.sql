ALTER TABLE job_descriptions
    ADD COLUMN raw_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE job_descriptions
    ADD COLUMN analysis_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (analysis_status IN ('pending', 'succeeded', 'failed'));

ALTER TABLE job_descriptions
    ADD COLUMN analysis_error TEXT NOT NULL DEFAULT '';

CREATE INDEX job_descriptions_updated_at_idx
    ON job_descriptions(updated_at DESC);
