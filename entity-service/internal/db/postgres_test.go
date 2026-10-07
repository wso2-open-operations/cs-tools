// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/License-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package db

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// TestNewPoolIfNeeded_ServiceNowWithNoDBUserSkipsPool covers the one case
// NewPoolIfNeeded must still skip: a local SN-mode setup with no Postgres
// provisioned at all (DB credentials absent) starts without a reachable database,
// regardless of DataSource.
func TestNewPoolIfNeeded_ServiceNowWithNoDBUserSkipsPool(t *testing.T) {
	pool, err := NewPoolIfNeeded(&config.Config{
		DataSource: config.DataSourceServiceNow,
	})
	if err != nil {
		t.Fatalf("NewPoolIfNeeded() = %v, want nil", err)
	}
	if pool != nil {
		t.Fatal("NewPoolIfNeeded() returned a pool with no DB credentials configured")
	}
}

// TestNewPoolIfNeeded_ServiceNowWithDBUserAttemptsPool is the regression
// test for the actual bug: gating purely on DataSource left every
// Postgres-only side table (alert_incident_mapping et al.) 404ing in any
// SN-mode deployment that DID have Postgres configured. Confirms the gate
// is DB credentials, not DataSource — a real connection attempt fires (and fails,
// since this host doesn't exist) rather than short-circuiting to (nil, nil).
func TestNewPoolIfNeeded_ServiceNowWithDBUserAttemptsPool(t *testing.T) {
	_, err := NewPoolIfNeeded(&config.Config{
		DataSource: config.DataSourceServiceNow,
		DBHost:     "db-that-does-not-exist.invalid",
		DBPort:     "5432",
		DBUser:     "user",
		DBPassword: "password",
		DBName:     "db",
	})
	if err == nil {
		t.Fatal("NewPoolIfNeeded() = nil error, want a dial failure — pool creation was not attempted")
	}
}

// The read pool's session parameters are what turn a mis-routed write into a
// loud SQLSTATE 25006 failure, so they are asserted on the built config
// without a database.
func TestPoolConfig_RuntimeParams(t *testing.T) {
	const dsn = "postgres://user:pw@db.invalid:5432/db?sslmode=disable"
	for _, tc := range []struct {
		name         string
		readOnly     bool
		wantReadOnly string
	}{
		{"write pool", false, ""},
		{"read pool", true, "on"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := poolConfig(dsn, 7, 3, time.Minute, 2*time.Minute, tc.readOnly)
			if err != nil {
				t.Fatalf("poolConfig: %v", err)
			}
			rp := cfg.ConnConfig.RuntimeParams
			if rp["jit"] != "off" {
				t.Errorf("jit = %q, want off", rp["jit"])
			}
			got, ok := rp["default_transaction_read_only"]
			if got != tc.wantReadOnly || ok != tc.readOnly {
				t.Errorf("default_transaction_read_only = %q (set=%v), want %q (set=%v)", got, ok, tc.wantReadOnly, tc.readOnly)
			}
			if cfg.MaxConns != 7 || cfg.MinConns != 3 || cfg.MaxConnLifetime != time.Minute || cfg.MaxConnIdleTime != 2*time.Minute {
				t.Errorf("sizing not applied: %d/%d/%s/%s", cfg.MaxConns, cfg.MinConns, cfg.MaxConnLifetime, cfg.MaxConnIdleTime)
			}
		})
	}
}

func TestPoolConfig_BadDSN(t *testing.T) {
	if _, err := poolConfig("://not a dsn", 1, 0, time.Minute, time.Minute, true); err == nil {
		t.Error("poolConfig accepted an unparseable DSN")
	}
}
