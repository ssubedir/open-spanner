ALTER TABLE auth_sessions
ADD COLUMN kind TEXT NOT NULL DEFAULT 'access' CHECK (kind IN ('access', 'refresh'));
