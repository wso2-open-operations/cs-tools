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

// This is an integration test: it asserts a schema FACT (relrowsecurity /
// relforcerowsecurity on pg_class) that can only be observed against a real
// Postgres, not reasoned about from Go code. It exists specifically as the
// regression guard the RLS migration series' own plan called for: nothing
// else in this codebase would notice if a future migration accidentally
// re-disabled RLS on one of these tables, or added a new caller-scoped
// table without FORCE, or a schema-owning role change (e.g. a future
// staging/production role swap) silently made FORCE moot. Same DSN and
// skip-when-unset pattern as every other integration test in this package
// (reuses caseStatsPool):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run RLSSchemaIntegration

package repository_test

import (
	"context"
	"testing"
)

// rlsProtectedTables is every table this migration series (000085,
// 0141-0151, minus sla/incident/incident_task/problem, dropped by 0153) put under FORCE ROW LEVEL SECURITY, gathered directly from
// the migrations/ directory rather than hand-maintained from memory --
// see this file's own test for how it's cross-checked against that
// directory. Keep this list and the migrations in sync: a table added here
// with no matching migration, or a migration adding FORCE without a
// matching entry here, is exactly the drift this test exists to catch.
var rlsProtectedTables = []string{
	"announcement",
	"case_escalation",
	"case_escalation_notification_list",
	"customer_call",
	"time_card",
	"time_card_approver",
	"change_request",
	"approval_stage",
	"approval_stage_approver",
	"conversation",
	"work_item",
	"case",
	"comment",
	"comment_edit_history",
	"case_attachment",
	"work_item_tag",
	"work_item_watcher",
	"work_item_activity",
	"engagement",
	"service_request",
	"security_report_analysis",
	"deployment",
	"deployed_product",
	"change_request_deployment",
	"change_request_deployed_product",
}

// TestRLSSchemaIntegration_EveryProtectedTableHasForceRowLevelSecurity is the
// guardrail the RLS migration series' own plan called for: a schema test
// asserting relrowsecurity/relforcerowsecurity are both true for every
// table this series protects. Checked against pg_class directly (not
// pg_tables, which only exposes relrowsecurity, not relforcerowsecurity) so
// a table whose RLS is enabled but not forced -- e.g. FORCE quietly dropped
// by a later migration, or never applied because the connecting role turned
// out to be a superuser/table owner in some environment -- is caught, not
// just outright disabled RLS.
func TestRLSSchemaIntegration_EveryProtectedTableHasForceRowLevelSecurity(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = ANY($1) AND n.nspname = current_schema()`,
		rlsProtectedTables,
	)
	if err != nil {
		t.Fatalf("query pg_class: %v", err)
	}
	defer rows.Close()

	found := make(map[string]struct{ enabled, forced bool }, len(rlsProtectedTables))
	for rows.Next() {
		var name string
		var enabled, forced bool
		if err := rows.Scan(&name, &enabled, &forced); err != nil {
			t.Fatalf("scan pg_class row: %v", err)
		}
		found[name] = struct{ enabled, forced bool }{enabled, forced}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pg_class rows: %v", err)
	}

	for _, table := range rlsProtectedTables {
		state, ok := found[table]
		if !ok {
			t.Errorf("table %q not found in current_schema() -- renamed, dropped, or this test's schema assumption (current_schema() matching the test DSN's own search_path) no longer holds", table)
			continue
		}
		if !state.enabled {
			t.Errorf("table %q: relrowsecurity = false, want true (ROW LEVEL SECURITY not enabled)", table)
		}
		if !state.forced {
			t.Errorf("table %q: relforcerowsecurity = false, want true (FORCE ROW LEVEL SECURITY not set -- a non-superuser table owner would bypass its own policies)", table)
		}
	}
}

// TestRLSSchemaIntegration_PolicyHelperFunctionsAreParallelSafe guards
// migration 0177. One PARALLEL UNSAFE function anywhere in a policy makes
// every query on that table non-parallel, which is what made internal callers
// several times slower than with RLS off. A later CREATE OR REPLACE FUNCTION
// without a PARALLEL clause silently resets the label, so nothing else would
// notice it regress.
func TestRLSSchemaIntegration_PolicyHelperFunctionsAreParallelSafe(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT DISTINCT p.proname, p.proparallel
		FROM pg_policy pol
		JOIN pg_depend d ON d.classid = 'pg_policy'::regclass AND d.objid = pol.oid
		                AND d.refclassid = 'pg_proc'::regclass
		JOIN pg_proc p ON p.oid = d.refobjid
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = current_schema()`)
	if err != nil {
		t.Fatalf("query policy function dependencies: %v", err)
	}
	defer rows.Close()

	seen := 0
	for rows.Next() {
		var name, parallel string
		if err := rows.Scan(&name, &parallel); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen++
		if parallel != "s" {
			t.Errorf("policy helper function %q has proparallel = %q, want 's' (PARALLEL SAFE): see migration 0177", name, parallel)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	if seen == 0 {
		t.Error("found no functions referenced by any policy; is_project_member should at least be one -- the query or the schema assumption changed")
	}
}

// rlsCommandsDeniedOnPurpose lists table/command pairs that intentionally have
// no policy, so the database refuses that command for every caller (internal
// ones included). Until migration 0190 seven such pairs existed, nobody had
// written down that they were deliberate, and the csm-sync-service -- which
// does run those commands -- had every one of them refused. To leave a pair
// without a policy, add it here with the reason, and the reason must say why
// the sync does not need it.
var rlsCommandsDeniedOnPurpose = map[string]map[string]string{
	// Our own table (migration 0191): nothing outside entity-service writes it
	// (the sync's change_request_product / change_request_product_version are
	// different tables), and entity-service only ever deletes and re-inserts
	// the whole list, so it never issues an UPDATE. change_request_deployment,
	// the sync's own junction, does have the UPDATE policy (migration 0205).
	"change_request_deployed_product": {
		"UPDATE": "entity-service deletes and re-inserts the snapshot; the sync never writes this table",
	},
}

// TestRLSSchemaIntegration_EveryProtectedTableHasAPolicyForEveryCommand
// guards against a table being left with RLS enabled and no policy for one of
// SELECT, INSERT, UPDATE or DELETE. With FORCE ROW LEVEL SECURITY a command
// that has no policy fails for everyone, and it fails quietly: an UPDATE
// affects zero rows, and an INSERT ... ON CONFLICT DO UPDATE raises
// "new row violates row-level security policy (USING expression)". Nothing
// else would notice a new table, or a dropped policy, creating that gap.
//
// The tables checked are discovered from the catalog (every table in the
// schema with RLS enabled), not taken from rlsProtectedTables, so a new RLS
// table cannot slip past this test by being left out of that list; the list
// is then cross-checked against what was discovered. Only a PERMISSIVE policy
// that applies to every role (TO public, the default) counts as coverage: a
// RESTRICTIVE policy only narrows what permissive ones grant, and a policy
// written TO one role leaves every other role without the command.
func TestRLSSchemaIntegration_EveryProtectedTableHasAPolicyForEveryCommand(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()
	commands := []string{"SELECT", "INSERT", "UPDATE", "DELETE"}

	tableRows, err := pool.Query(ctx, `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema() AND c.relkind IN ('r', 'p') AND c.relrowsecurity
		ORDER BY c.relname`)
	if err != nil {
		t.Fatalf("query RLS-enabled tables: %v", err)
	}
	defer tableRows.Close()
	var discovered []string
	for tableRows.Next() {
		var name string
		if err := tableRows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		discovered = append(discovered, name)
	}
	if err := tableRows.Err(); err != nil {
		t.Fatalf("iterate RLS-enabled tables: %v", err)
	}
	if len(discovered) == 0 {
		t.Fatal("found no table with row-level security enabled in current_schema(); the query or the schema assumption changed")
	}

	listed := make(map[string]bool, len(rlsProtectedTables))
	for _, name := range rlsProtectedTables {
		listed[name] = true
	}
	for _, name := range discovered {
		if !listed[name] {
			t.Errorf("table %q has row-level security enabled but is missing from rlsProtectedTables: add it so the FORCE and per-table checks cover it", name)
		}
	}

	rows, err := pool.Query(ctx, `
		SELECT tablename, cmd
		FROM pg_policies
		WHERE schemaname = current_schema()
		  AND permissive = 'PERMISSIVE'
		  AND 'public' = ANY (roles)`)
	if err != nil {
		t.Fatalf("query pg_policies: %v", err)
	}
	defer rows.Close()

	covered := make(map[string]map[string]bool, len(discovered))
	for rows.Next() {
		var table, cmd string
		if err := rows.Scan(&table, &cmd); err != nil {
			t.Fatalf("scan pg_policies row: %v", err)
		}
		if covered[table] == nil {
			covered[table] = make(map[string]bool, len(commands))
		}
		if cmd == "ALL" {
			for _, c := range commands {
				covered[table][c] = true
			}
		} else {
			covered[table][cmd] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pg_policies rows: %v", err)
	}

	for _, table := range discovered {
		for _, cmd := range commands {
			if covered[table][cmd] {
				continue
			}
			if reason, ok := rlsCommandsDeniedOnPurpose[table][cmd]; ok && reason != "" {
				continue
			}
			t.Errorf("table %q has no permissive %s policy for every role: with row-level security enabled that command is refused for every caller, internal ones included. Add a policy (see migration 0190 for the internal-only shape) or list the pair in rlsCommandsDeniedOnPurpose with the reason", table, cmd)
		}
	}
}
