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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.
package server

import (
	"os"
	"regexp"
	"testing"
)

// TestAuditLogMaintenanceRouteIsOffByDefaultAndServiceOnly pins two things about
// POST /audit-log/maintenance: it is registered only behind CSM_AUDIT_LOG_ENABLED
// (so an unconfigured deployment answers 404 and runs nothing), and it is
// service-credential only (a user token, internal or not, gets 403).
func TestAuditLogMaintenanceRouteIsOffByDefaultAndServiceOnly(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}

	gated := regexp.MustCompile(`if db != nil && cfg\.AuditLogEnabled \{[^}]*HandleFunc\("POST /audit-log/maintenance",\s*serviceOnly\(`)
	if !gated.Match(src) {
		t.Error(`POST /audit-log/maintenance must be registered inside "if db != nil && cfg.AuditLogEnabled" and wrapped in serviceOnly`)
	}
}
