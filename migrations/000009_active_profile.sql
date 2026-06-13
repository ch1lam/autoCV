ALTER TABLE profiles
    ADD COLUMN is_active INTEGER NOT NULL DEFAULT 0
        CHECK (is_active IN (0, 1));

UPDATE profiles
   SET is_active = 1
 WHERE id = (
    SELECT id
      FROM profiles
     ORDER BY created_at, id
     LIMIT 1
 );

CREATE UNIQUE INDEX profiles_one_active_idx
    ON profiles(is_active)
    WHERE is_active = 1;
