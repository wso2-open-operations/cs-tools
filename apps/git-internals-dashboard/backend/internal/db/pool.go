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

// Package db holds the pgxpool connection pool and every hand-written SQL
// query the backend issues (no ORM, no sqlc). Package-level helpers here
// are transport-agnostic infrastructure only; domain writes live in
// internal/ingest, internal/sync, and internal/jobs.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/appconfig"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a pgxpool.Pool for databaseURL and verifies connectivity
// with a Ping before returning, so a misconfigured DSN fails at boot rather
// than on the first request. Equivalent to NewPoolWithConfig with every
// tuning knob left at pgx's own default.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return NewPoolWithConfig(ctx, databaseURL, appconfig.Database{})
}

// NewPoolWithConfig creates a pgxpool.Pool for databaseURL, applying each
// non-nil field of dbCfg over pgx's own defaults, and verifies connectivity
// with a Ping before returning.
func NewPoolWithConfig(ctx context.Context, databaseURL string, dbCfg appconfig.Database) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	if dbCfg.MaxConns != nil {
		poolCfg.MaxConns = *dbCfg.MaxConns
	}
	if dbCfg.MinConns != nil {
		poolCfg.MinConns = *dbCfg.MinConns
	}
	if dbCfg.MaxConnLifetimeMinutes != nil {
		poolCfg.MaxConnLifetime = time.Duration(*dbCfg.MaxConnLifetimeMinutes) * time.Minute
	}
	if dbCfg.MaxConnIdleTimeMinutes != nil {
		poolCfg.MaxConnIdleTime = time.Duration(*dbCfg.MaxConnIdleTimeMinutes) * time.Minute
	}
	if dbCfg.HealthCheckPeriodSeconds != nil {
		poolCfg.HealthCheckPeriod = time.Duration(*dbCfg.HealthCheckPeriodSeconds) * time.Second
	}
	if dbCfg.ConnectTimeoutSeconds != nil {
		poolCfg.ConnConfig.ConnectTimeout = time.Duration(*dbCfg.ConnectTimeoutSeconds) * time.Second
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}
