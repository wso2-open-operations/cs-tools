// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package db manages the PostgreSQL connection pool for the entity service.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// NewPool creates a pgxpool connection pool for the given DSN, pings the
// database to confirm connectivity, and returns the pool ready for use.
// The caller is responsible for calling pool.Close on shutdown.
//
// maxConns/minConns/maxConnLifetime/maxConnIdleTime were fixed constants
// here (20/2/30m/5m) until they became env-configurable
// (DB_POOL_MAX_CONNS/DB_POOL_MIN_CONNS/DB_POOL_MAX_CONN_LIFETIME/
// DB_POOL_MAX_CONN_IDLE_TIME, see config.Config's own doc comments for
// those fields) — config.Load() already applies those same four values as
// its defaults when the corresponding env var is unset, so this function
// itself carries no defaults of its own any more and always receives a
// real, non-zero value from every caller.
func NewPool(ctx context.Context, dsn string, maxConns, minConns int32, maxConnLifetime, maxConnIdleTime time.Duration) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	// JIT off: row-level-security policies inflate the planner's cost
	// estimates into the millions even for queries that touch a few rows, which
	// crosses jit_above_cost and makes Postgres compile ~100 functions per
	// request. Measured on the real-data copy, that compile time was 60-90% of
	// the latency of cases/search and global search (e.g. 1.5s with JIT vs
	// 0.38s without). These are short OLTP queries; JIT never pays for itself.
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["jit"] = "off"

	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.MaxConnIdleTime = maxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// NewPoolIfNeeded creates a Postgres connection pool when database credentials
// are configured, whatever the data source.
//
// Gated on whether DB credentials are actually configured (cfg.HasDatabase()),
// not on cfg.DataSource. DATA_SOURCE=servicenow only means case/account/etc.
// reads go through the SN integration service instead of this pool — it says
// nothing about whether Postgres itself is available. Product consumption keeps
// its provisioning state in Postgres and dual-writes it alongside ServiceNow, so
// a DATA_SOURCE=servicenow deployment -- which is what staging and production run
// -- still needs a pool. Side tables (event_publish_failures, sla_clocks,
// scheduled_task_run, alert_incident_mapping) have no ServiceNow equivalent and
// are always backed by Postgres regardless of DATA_SOURCE; gating on DataSource
// alone left them 404ing in any SN-mode deployment that had a perfectly good
// Postgres instance configured right next to it, simply unused.
//
// Gating on HasDatabase preserves the one thing DataSource-gating was actually
// protecting: a local SN-mode setup with no Postgres provisioned at all still
// gets (nil, nil), exactly as before — see config.Config.Validate's doc comment,
// which is why DB_USER/DB_PASSWORD/DB_NAME stay optional (not required) for
// DATA_SOURCE=servicenow rather than becoming mandatory here.
func NewPoolIfNeeded(cfg *config.Config) (*pgxpool.Pool, error) {
	if !cfg.HasDatabase() {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return NewPool(ctx, cfg.DSN(), cfg.DBPoolMaxConns, cfg.DBPoolMinConns, cfg.DBPoolMaxConnLifetime, cfg.DBPoolMaxConnIdleTime)
}
