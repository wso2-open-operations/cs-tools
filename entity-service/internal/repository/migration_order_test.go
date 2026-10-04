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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// orderedMigrationFiles is the Go copy of scripts/migration_order.sh, for the
// tests that build a schema themselves (plg_schema_test.go). The script's
// header explains the rules; TestMigrationOrder_MatchesScript keeps the two
// copies identical.
func orderedMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fourDigit := regexp.MustCompile(`^[0-9]{4}_.*\.sql$`)
	sixDigitUp := regexp.MustCompile(`^[0-9]{6}_.*\.up\.sql$`)
	var four, six []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch name := e.Name(); {
		case fourDigit.MatchString(name):
			four = append(four, name)
		case sixDigitUp.MatchString(name):
			six = append(six, name)
		}
	}
	// Within one width every name starts with a fixed-width number, so a
	// byte-wise sort is the script's `LC_ALL=C sort -V` order.
	sort.Strings(four)
	sort.Strings(six)

	const stragglersBefore = 149
	var ordered []string
	add := func(names []string, keep func(int) bool) {
		for _, name := range names {
			n, _ := strconv.Atoi(name[:strings.IndexByte(name, '_')])
			if keep(n) && !supersededMigrations[strings.TrimSuffix(name, ".sql")] {
				ordered = append(ordered, filepath.Join(dir, name))
			}
		}
	}
	add(four, func(n int) bool { return n < stragglersBefore })
	add(six, func(int) bool { return true })
	add(four, func(n int) bool { return n >= stragglersBefore })
	return ordered, nil
}

var supersededMigrations = map[string]bool{
	"0152_team_schedule_tables":                true,
	"0153_team_schedule_catalogue":             true,
	"0154_team_schedule_vocabulary":            true,
	"0155_schedule_customer_allocation_split":  true,
	"0156_schedule_shift_is_rotation":          true,
	"0157_schedule_shift_required_headcount":   true,
	"0158_schedule_assignment_activity":        true,
	"0159_schedule_absence_activity":           true,
	"0160_schedule_absence_kind_consolidation": true,
	"0161_schedule_americas_weekend_names":     true,
	"0162_schedule_remove_invented_windows":    true,
	"0163_schedule_rotation_short_codes":       true,
	"0164_schedule_integrity_constraints":      true,
	"0165_schedule_team_key_catalogue":         true,
	"0166_schedule_absence_bucket_enum":        true,
	"0167_team_schedule_table_prefix":          true,
	"0168_team_schedule_audit":                 true,
}

func TestMigrationOrder_RulesHoldOnTheRealDirectory(t *testing.T) {
	files, err := orderedMigrationFiles(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, f := range files {
		name := filepath.Base(f)
		if strings.HasSuffix(name, ".down.sql") {
			t.Errorf("%s is a rollback and must never be applied", name)
		}
		if supersededMigrations[strings.TrimSuffix(name, ".sql")] {
			t.Errorf("%s is superseded and must be skipped", name)
		}
		pos[name] = i
	}
	mustPrecede := [][2]string{
		{"0001_control_plane.sql", "000085_announcement_visibility_rls.up.sql"},
		{"0088_announcement_add_announcement_type.sql", "000085_announcement_visibility_rls.up.sql"},
		{"000085_announcement_visibility_rls.up.sql", "0149_announcement_security_fallback_no_contact.sql"},
		{"0083_outage_table.sql", "000087_cloud_status_outage_outbox.up.sql"},
		{"0153_team_schedule_tables.sql", "0156_team_schedule_rota_admin_roles.sql"},
	}
	for _, p := range mustPrecede {
		a, okA := pos[p[0]]
		b, okB := pos[p[1]]
		if !okA || !okB {
			// A rename upstream: only the order check is meaningless, not wrong.
			t.Logf("skipping %s < %s: not both present", p[0], p[1])
			continue
		}
		if a >= b {
			t.Errorf("%s (#%d) must be applied before %s (#%d)", p[0], a, p[1], b)
		}
	}
}

func TestMigrationOrder_MatchesScript(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	root := filepath.Join("..", "..")
	cmd := exec.Command(bash, filepath.Join("scripts", "migration_order.sh")) // #nosec G204 -- fixed repo script, no caller input
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("scripts/migration_order.sh: %v", err)
	}
	want := strings.Fields(string(out))
	gotRel, err := orderedMigrationFiles(filepath.Join(root, "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(gotRel) != len(want) {
		t.Fatalf("Go order has %d files, script has %d", len(gotRel), len(want))
	}
	for i := range want {
		if filepath.Base(gotRel[i]) != filepath.Base(want[i]) {
			t.Fatalf("position %d: Go order %s, script %s", i, filepath.Base(gotRel[i]), filepath.Base(want[i]))
		}
	}
}
