CREATE TYPE user_type AS ENUM ('internal', 'customer');

CREATE TABLE IF NOT EXISTS users (
    id          TEXT        PRIMARY KEY,
    user_name   TEXT        NOT NULL UNIQUE,
    first_name  TEXT        NOT NULL,
    last_name   TEXT        NOT NULL,
    email       TEXT        NOT NULL UNIQUE,
    phone       TEXT,
    timezone    TEXT,
    user_type   user_type   NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_user_type   ON users (user_type);
CREATE INDEX IF NOT EXISTS idx_users_created_at  ON users (created_at DESC);

CREATE TYPE account_tier AS ENUM ('basic', 'enterprise');

CREATE TABLE IF NOT EXISTS accounts (
    id                    TEXT          PRIMARY KEY,
    sf_id                 TEXT          NOT NULL UNIQUE,
    name                  TEXT          NOT NULL,
    tier                  account_tier  NOT NULL,
    region                TEXT,
    activation_date       TIMESTAMPTZ   NOT NULL,
    deactivation_date     TIMESTAMPTZ,
    owner_id              TEXT          NOT NULL,
    technical_owner_id    TEXT,
    agent_enabled         BOOLEAN       NOT NULL DEFAULT FALSE,
    kb_references_enabled BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accounts_created_at ON accounts (created_at DESC);
