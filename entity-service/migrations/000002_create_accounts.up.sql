CREATE TYPE account_tier AS ENUM ('basic', 'enterprise');

CREATE TABLE IF NOT EXISTS accounts (
    id                    TEXT         PRIMARY KEY,
    sf_id                 TEXT         NOT NULL UNIQUE,
    name                  TEXT         NOT NULL,
    tier                  account_tier NOT NULL,
    region                TEXT,
    activation_date       TIMESTAMPTZ  NOT NULL,
    deactivation_date     TIMESTAMPTZ,
    owner_id              TEXT         NOT NULL,
    technical_owner_id    TEXT,
    agent_enabled         BOOLEAN      NOT NULL DEFAULT FALSE,
    kb_references_enabled BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_accounts_owner_id           FOREIGN KEY (owner_id)           REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT fk_accounts_technical_owner_id FOREIGN KEY (technical_owner_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_accounts_created_at ON accounts (created_at DESC);
