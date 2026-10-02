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

package db

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// fakeRLSRow lets a test hand CheckRLSProtection the four values the real query returns.
type fakeRLSRow struct {
	role    string
	bypass  bool
	tables  int
	exempt  int
	scanErr error
}

func (r fakeRLSRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = r.role
	*(dest[1].(*bool)) = r.bypass
	*(dest[2].(*int)) = r.tables
	*(dest[3].(*int)) = r.exempt
	return nil
}

type fakeRLSQuerier struct{ row fakeRLSRow }

func (f fakeRLSQuerier) QueryRow(context.Context, string, ...any) pgx.Row { return f.row }

func TestRLSProtectionVerdict(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		p          RLSProtection
		wantInUse  bool
		wantExempt bool
	}{
		{"RLS not in use: nothing to be exempt from", RLSProtection{Role: "app"}, false, false},
		{"superuser is exempt", RLSProtection{Role: "postgres", Tables: 23, RoleBypassesAll: true}, true, true},
		{"BYPASSRLS role is exempt", RLSProtection{Role: "staff", Tables: 23, RoleBypassesAll: true}, true, true},
		{"owner without FORCE is exempt", RLSProtection{Role: "owner", Tables: 23, OwnerExemptTables: 23}, true, true},
		{"owner exempt on just one table is still exempt", RLSProtection{Role: "owner", Tables: 23, OwnerExemptTables: 1}, true, true},
		{"owner with FORCE is bound", RLSProtection{Role: "owner", Tables: 23}, true, false},
		{"ordinary non-owner role is bound", RLSProtection{Role: "app", Tables: 23}, true, false},
		{"bypass role with RLS not in use is not flagged", RLSProtection{Role: "postgres", RoleBypassesAll: true}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.p.InUse(); got != tc.wantInUse {
				t.Errorf("InUse() = %v, want %v", got, tc.wantInUse)
			}
			if got := tc.p.Exempt(); got != tc.wantExempt {
				t.Errorf("Exempt() = %v, want %v", got, tc.wantExempt)
			}
			if tc.p.Summary() == "" {
				t.Error("Summary() is empty")
			}
		})
	}
}

func TestVerifyRLSProtection(t *testing.T) {
	t.Parallel()
	exempt := fakeRLSQuerier{fakeRLSRow{role: "owner", tables: 23, exempt: 23}}
	bound := fakeRLSQuerier{fakeRLSRow{role: "app", tables: 23}}

	if err := VerifyRLSProtection(context.Background(), exempt, false); err != nil {
		t.Errorf("an exempt role must only be logged when not required, got %v", err)
	}
	err := VerifyRLSProtection(context.Background(), exempt, true)
	if err == nil {
		t.Fatal("an exempt role must fail startup when required")
	}
	if !strings.Contains(err.Error(), "NOT isolated") || !strings.Contains(err.Error(), "owner") {
		t.Errorf("the error should say customers are not isolated and name the role, got %q", err)
	}
	if err := VerifyRLSProtection(context.Background(), bound, true); err != nil {
		t.Errorf("a bound role must pass even when required, got %v", err)
	}
	// The check failing is never a reason to stop the service.
	broken := fakeRLSQuerier{fakeRLSRow{scanErr: errors.New("permission denied for pg_roles")}}
	if err := VerifyRLSProtection(context.Background(), broken, true); err != nil {
		t.Errorf("a check that cannot run must not fail startup, got %v", err)
	}
}
