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

const (
	poolMaxConns        int32         = 20               // maximum open connections in the pool
	poolMinConns        int32         = 2                // connections kept warm when idle
	poolMaxConnLifetime time.Duration = 30 * time.Minute // rotate connections to avoid stale server-side state
	poolMaxConnIdleTime time.Duration = 5 * time.Minute  // release unused connections back to the OS
)

// NewPool creates a pgxpool connection pool for the given DSN, pings the
// database to confirm connectivity, and returns the pool ready for use.
// The caller is responsible for calling pool.Close on shutdown.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	cfg.MaxConns = poolMaxConns
	cfg.MinConns = poolMinConns
	cfg.MaxConnLifetime = poolMaxConnLifetime
	cfg.MaxConnIdleTime = poolMaxConnIdleTime

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
	return NewPool(ctx, cfg.DSN())
}
