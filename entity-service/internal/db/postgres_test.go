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

func TestNewPoolIfNeeded_ServiceNowSkipsPool(t *testing.T) {
	pool, err := NewPoolIfNeeded(&config.Config{
		DataSource: config.DataSourceServiceNow,
		DBHost:     "db-that-must-not-be-dialed.example",
		DBPort:     "5432",
		DBUser:     "user",
		DBPassword: "password",
		DBName:     "db",
	})
	if err != nil {
		t.Fatalf("NewPoolIfNeeded() = %v, want nil", err)
	}
	if pool != nil {
		t.Fatal("NewPoolIfNeeded() returned a pool for DATA_SOURCE=servicenow")
	}
}
