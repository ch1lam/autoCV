ALTER TABLE artifacts
    ADD COLUMN preview_paths_json TEXT NOT NULL DEFAULT '[]';
