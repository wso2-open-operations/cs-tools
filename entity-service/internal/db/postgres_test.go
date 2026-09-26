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
