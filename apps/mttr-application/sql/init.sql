-- MTTR Application Database Schema
-- Run this script to initialize the database

-- ============================================
-- Table: case_events (raw case data from ServiceNow)
-- ============================================
CREATE TABLE IF NOT EXISTS case_events (
    id                   SERIAL PRIMARY KEY,
    case_sys_id          VARCHAR(32) UNIQUE NOT NULL,
    product              VARCHAR(100) NOT NULL,
    cs_team              VARCHAR(100) NOT NULL,
    business_duration_ms BIGINT NOT NULL,
    created_date         TIMESTAMP WITH TIME ZONE NOT NULL,
    closed_date          TIMESTAMP WITH TIME ZONE,
    case_type            VARCHAR(50) NOT NULL,
    priority             VARCHAR(30),
    is_patched           BOOLEAN NOT NULL DEFAULT FALSE,
    case_state           VARCHAR(30) NOT NULL,
    ingested_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for common query patterns used by aggregation
CREATE INDEX IF NOT EXISTS idx_case_events_closed_date ON case_events(closed_date);
CREATE INDEX IF NOT EXISTS idx_case_events_case_type ON case_events(case_type);
CREATE INDEX IF NOT EXISTS idx_case_events_cs_team ON case_events(cs_team);
CREATE INDEX IF NOT EXISTS idx_case_events_priority ON case_events(priority);
CREATE INDEX IF NOT EXISTS idx_case_events_case_state ON case_events(case_state);
CREATE INDEX IF NOT EXISTS idx_case_events_is_patched ON case_events(is_patched);
CREATE INDEX IF NOT EXISTS idx_case_events_product ON case_events(product);

-- Composite index for the most common aggregation query
CREATE INDEX IF NOT EXISTS idx_case_events_state_closed ON case_events(case_state, closed_date);

-- ============================================
-- Table: mttr_cache (pre-computed aggregations)
-- ============================================
CREATE TABLE IF NOT EXISTS mttr_cache (
    id               SERIAL PRIMARY KEY,
    cache_key        VARCHAR(255) UNIQUE NOT NULL,
    dimension_type   VARCHAR(50) NOT NULL,
    dimension_labels JSONB NOT NULL,
    period_start     DATE NOT NULL,
    period_end       DATE NOT NULL,
    total_cases      INTEGER NOT NULL,
    excluded_cases   INTEGER NOT NULL,
    p95_cutoff_ms    BIGINT,
    truncated_avg_ms BIGINT NOT NULL,
    simple_avg_ms    BIGINT NOT NULL,
    min_sample_met   BOOLEAN NOT NULL DEFAULT TRUE,
    calculated_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mttr_cache_dimension_type ON mttr_cache(dimension_type);
CREATE INDEX IF NOT EXISTS idx_mttr_cache_key ON mttr_cache(cache_key);

-- ============================================
-- Table: ingestion_log (audit trail for batch ingestions)
-- ============================================
CREATE TABLE IF NOT EXISTS ingestion_log (
    id                SERIAL PRIMARY KEY,
    batch_id          VARCHAR(100) NOT NULL,
    records_received  INTEGER NOT NULL,
    records_inserted  INTEGER NOT NULL,
    records_updated   INTEGER NOT NULL,
    records_rejected  INTEGER NOT NULL,
    rejected_details  JSONB,
    ingested_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ingestion_log_batch_id ON ingestion_log(batch_id);
CREATE INDEX IF NOT EXISTS idx_ingestion_log_ingested_at ON ingestion_log(ingested_at);

-- ============================================
-- Table: case_events_summary (pre-computed 3-month aggregates for archived data)
-- ============================================
-- When case_events rows age past RETENTION_CASE_EVENTS_MONTHS (default 24),
-- the retention job summarises them into this table (one row per unique
-- combination of quarter period, case_type, priority, cs_team, product,
-- is_patched) before deleting the raw rows.
--
-- Each row stores the P95 truncated-mean MTTR that was computed from the
-- raw durations at summarisation time.  This is the only place historical
-- MTTR survives after deletion from case_events.
CREATE TABLE IF NOT EXISTS case_events_summary (
    id               SERIAL PRIMARY KEY,
    period_label     VARCHAR(20) NOT NULL,     -- e.g. "2025-Q1", "2025-Q2", "2025-Q3", "2025-Q4"
    period_start     DATE NOT NULL,
    period_end       DATE NOT NULL,
    case_type        VARCHAR(50) NOT NULL,     -- Incident / Query
    priority         VARCHAR(30),              -- P1..P4 (NULL for Queries)
    cs_team          VARCHAR(100) NOT NULL,
    product          VARCHAR(100),             -- NULL = "all products" roll-up
    is_patched       BOOLEAN,                  -- NULL = "both" roll-up
    total_cases      INTEGER NOT NULL,
    excluded_cases   INTEGER NOT NULL,
    p95_cutoff_ms    BIGINT,
    truncated_avg_ms BIGINT NOT NULL,
    simple_avg_ms    BIGINT NOT NULL,
    min_sample_met   BOOLEAN NOT NULL DEFAULT TRUE,
    summarised_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Prevent duplicate summaries for the same slice
CREATE UNIQUE INDEX IF NOT EXISTS idx_ces_unique_slice
    ON case_events_summary (
        period_label, case_type,
        COALESCE(priority, ''), cs_team,
        COALESCE(product, ''), COALESCE(is_patched::text, '')
    );

CREATE INDEX IF NOT EXISTS idx_ces_period ON case_events_summary(period_start, period_end);
CREATE INDEX IF NOT EXISTS idx_ces_case_type ON case_events_summary(case_type);
CREATE INDEX IF NOT EXISTS idx_ces_team ON case_events_summary(cs_team);
-- Composite for /summary/historical: WHERE case_type = $1 ORDER BY period_start, …
-- Lets both the filter AND the primary sort come from a single index
-- scan — no in-memory sort step for period_start. Progressively more
-- valuable as case_events_summary grows (retained forever).
CREATE INDEX IF NOT EXISTS idx_ces_case_type_period ON case_events_summary(case_type, period_start);
