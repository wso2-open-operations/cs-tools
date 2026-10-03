// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package postgres connects to Azure Flexible Server PostgreSQL and claims id ranges from alert_seq and writes alerts rows.
package postgres

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const (
	// IDPrefix and IDWidth must match alerts-core's poller, which rebuilds ids from sequence numbers.
	IDPrefix = "ALT"
	IDWidth  = 9
)

// FormatID renders a sequence number as an alert id, e.g. 123 -> "ALT000000123".
func FormatID(seq int64) string {
	return fmt.Sprintf("%s%0*d", IDPrefix, IDWidth, seq)
}

// Config holds connection settings from PG* env vars, identical to alerts-core's.
type Config struct {
	Host     string `env:"PGHOST,notEmpty"`
	Port     int    `env:"PGPORT" envDefault:"5432"`
	Database string `env:"PGDATABASE,notEmpty"`
	User     string `env:"PGUSER,notEmpty"`
	Password string `env:"PGPASSWORD,notEmpty"`
	SSLMode  string `env:"PGSSLMODE" envDefault:"require"`
	// PoolMaxConns caps this replica's pgxpool connections; 0 leaves pgx's default, too small under concurrent load.
	PoolMaxConns int32 `env:"PGPOOLMAXCONNS" envDefault:"0"`
}

// ConfigFromEnv reads Config from the environment.
func ConfigFromEnv() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("postgres config: %w", err)
	}
	return cfg, nil
}

// Connect opens a pooled connection, bounding connect time and the default per-query timeout.
func Connect(cfg Config, connectTimeout, queryTimeout time.Duration) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(cfg.User), url.QueryEscape(cfg.Password), cfg.Host, cfg.Port, cfg.Database, cfg.SSLMode)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	poolCfg.ConnConfig.ConnectTimeout = connectTimeout
	if cfg.PoolMaxConns > 0 {
		poolCfg.MaxConns = cfg.PoolMaxConns
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	pingCtx, pingCancel := context.WithTimeout(context.Background(), queryTimeout)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// alertRow is the GORM model for the alerts table.
type alertRow struct {
	ID        string         `gorm:"column:id;primaryKey"`
	Source    string         `gorm:"column:source"`
	Alert     datatypes.JSON `gorm:"column:alert"`
	CreatedAt time.Time      `gorm:"column:created_at"`
}

// TableName pins alertRow to the alerts table instead of GORM's pluralized default.
func (alertRow) TableName() string { return "alerts" }

// Store implements the allocator's storage operations against one pool.
type Store struct {
	db           *gorm.DB
	queryTimeout time.Duration
	claimTimeout time.Duration
}

// NewStore wraps pool in a GORM DB and returns a Store over it.
func NewStore(pool *pgxpool.Pool, queryTimeout, claimTimeout time.Duration) (*Store, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm db: %w", err)
	}
	return &Store{db: db, queryTimeout: queryTimeout, claimTimeout: claimTimeout}, nil
}

// ClaimRange reserves n consecutive alert_seq values in one round trip and returns the first.
func (s *Store) ClaimRange(ctx context.Context, n int) (start int64, err error) {
	ctx, cancel := context.WithTimeout(ctx, s.claimTimeout)
	defer cancel()
	var ids []int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT nextval('alert_seq') FROM generate_series(1, ?)`, n,
	).Scan(&ids).Error; err != nil {
		return 0, fmt.Errorf("claim %d ids: %w", n, err)
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("claim %d ids: no rows returned", n)
	}
	return ids[0], nil
}

// Insert writes one alerts row. created_at is informational; alerts-core orders by id.
func (s *Store) Insert(ctx context.Context, id, source string, alert []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	row := alertRow{ID: id, Source: source, Alert: datatypes.JSON(alert), CreatedAt: time.Now().UTC()}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("insert %s: %w", id, err)
	}
	return nil
}

// InsertRow is one alerts row for InsertBatch.
type InsertRow struct {
	ID     string
	Source string
	Alert  []byte
}

// InsertBatch writes every row in one multi-row INSERT; a conflict or error fails the whole batch, so
// every returned error is the same shared error and callers retry each row individually.
func (s *Store) InsertBatch(ctx context.Context, rows []InsertRow) []error {
	now := time.Now().UTC()
	gormRows := make([]alertRow, len(rows))
	for i, r := range rows {
		gormRows[i] = alertRow{ID: r.ID, Source: r.Source, Alert: datatypes.JSON(r.Alert), CreatedAt: now}
	}
	errs := make([]error, len(rows))
	if err := s.db.WithContext(ctx).Create(&gormRows).Error; err != nil {
		wrapped := fmt.Errorf("insert batch of %d: %w", len(rows), err)
		for i := range errs {
			errs[i] = wrapped
		}
	}
	return errs
}

// InsertFiller writes a filler row only if id has no row yet; when not applied, existing is that row's alert column.
func (s *Store) InsertFiller(ctx context.Context, id, source, filler string) (applied bool, existing string, err error) {
	ctx, cancel := context.WithTimeout(ctx, s.claimTimeout)
	defer cancel()
	row := alertRow{ID: id, Source: source, Alert: datatypes.JSON(`"` + filler + `"`), CreatedAt: time.Now().UTC()}
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return false, "", fmt.Errorf("insert filler %s: %w", id, result.Error)
	}
	if result.RowsAffected == 1 {
		return true, "", nil
	}
	// Row already existed; read it back to report what's there.
	readCtx, readCancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer readCancel()
	var got alertRow
	if readErr := s.db.WithContext(readCtx).Select("alert").First(&got, "id = ?", id).Error; readErr != nil {
		return false, "", fmt.Errorf("insert filler %s: read existing: %w", id, readErr)
	}
	return false, string(got.Alert), nil
}
