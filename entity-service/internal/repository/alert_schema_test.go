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

package repository

import (
	"context"
	"strings"
	"testing"
)

func TestAlertSchemaProblem(t *testing.T) {
	ready := AlertSchema{HasSeq: true, HasTable: true, CanUseSeq: true, CanInsert: true}
	tests := map[string]struct {
		change func(*AlertSchema)
		want   string
	}{
		"ready":                {func(*AlertSchema) {}, ""},
		"nothing migrated":     {func(s *AlertSchema) { *s = AlertSchema{} }, "sequence alert_seq not found, table alert not found"},
		"sequence not granted": {func(s *AlertSchema) { s.CanUseSeq = false }, "no USAGE on alert_seq"},
		"table not granted":    {func(s *AlertSchema) { s.CanInsert = false }, "no INSERT on alert"},
	}
	for name, tt := range tests {
		s := ready
		tt.change(&s)
		if got := s.Problem(); got != tt.want {
			t.Errorf("%s: Problem() = %q, want %q", name, got, tt.want)
		}
	}
}

func TestAlertSchemaWhere(t *testing.T) {
	got := AlertSchema{Database: "csm_platform", User: "app", SearchPath: "app,public"}.Where()
	if want := `database "csm_platform" as "app" with search_path "app,public"`; got != want {
		t.Errorf("Where() = %q, want %q", got, want)
	}
}

func TestCheckAlertSchemaIntegration(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	schema, err := CheckAlertSchema(context.Background(), pool)
	if err != nil {
		t.Fatalf("CheckAlertSchema: %v", err)
	}
	if problem := schema.Problem(); problem != "" {
		t.Fatalf("Problem() = %q on a migrated database, want none (%s)", problem, schema.Where())
	}
	if schema.Database == "" || schema.User == "" || !strings.Contains(schema.Where(), schema.Database) {
		t.Errorf("Where() = %q, want the database and user filled in", schema.Where())
	}
}
