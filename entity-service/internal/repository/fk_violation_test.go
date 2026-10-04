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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestFKViolationError_NamesFieldWithoutDetail(t *testing.T) {
	detail := `Key (project_id)=(00000000-0000-0000-0000-000000000000) is not present in table "project".`
	cases := []struct {
		constraint, want string
	}{
		{"work_item_project_id_fkey", "one or more referenced IDs do not exist: projectId"},
		{"time_card_approver_approver_id_fkey", "one or more referenced IDs do not exist: approverIds"},
		{"some_unmapped_fkey", "one or more referenced IDs do not exist"},
	}
	for _, tc := range cases {
		err := fkViolationError(&pgconn.PgError{Code: "23503", ConstraintName: tc.constraint, Detail: detail}, "one or more referenced IDs do not exist")
		if err.Msg != tc.want {
			t.Errorf("%s: Msg = %q, want %q", tc.constraint, err.Msg, tc.want)
		}
		if strings.Contains(err.Msg, "Key (") || strings.Contains(err.Msg, "table") {
			t.Errorf("%s: Msg %q echoes the database detail", tc.constraint, err.Msg)
		}
	}
}

// No validation or conflict message in these files may be built from
// pgErr.Detail again.
func TestNoPgErrDetailInClientErrors(t *testing.T) {
	for _, f := range []string{"case_repo.go", "time_card_repo.go"} {
		src, err := os.ReadFile(f) // #nosec G304 -- fixed file names in this package
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "pgErr.Detail") {
			t.Errorf("%s builds an error from pgErr.Detail; use fkViolationError", f)
		}
	}
}
