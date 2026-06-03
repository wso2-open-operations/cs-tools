CREATE TYPE user_type_enum AS ENUM ('internal', 'customer', 'system');

CREATE TABLE IF NOT EXISTS users (
    id          TEXT        PRIMARY KEY,
    user_name   TEXT        NOT NULL UNIQUE,
    first_name  TEXT        NOT NULL,
    last_name   TEXT        NOT NULL,
    email       TEXT        NOT NULL UNIQUE,
    phone       TEXT,
    timezone    TEXT,
    user_type   user_type_enum   NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_user_type  ON users (user_type);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users (created_at DESC);
