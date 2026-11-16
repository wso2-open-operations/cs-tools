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
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// Both cases share SQLSTATE 42501. Only the insert policy refusing the row is
// the caller's 404; a missing privilege on the number sequence is a
// deployment problem and must not read as "project not found".
func TestIsRowSecurityRefusal(t *testing.T) {
	cases := map[string]struct {
		err  *pgconn.PgError
		want bool
	}{
		"insert policy":      {&pgconn.PgError{Code: "42501", Message: `new row violates row-level security policy for table "deployed_product"`}, true},
		"sequence privilege": {&pgconn.PgError{Code: "42501", Message: "permission denied for sequence ibitm_number_seq"}, false},
		"table privilege":    {&pgconn.PgError{Code: "42501", Message: "permission denied for table deployed_product"}, false},
		"other code":         {&pgconn.PgError{Code: "23505", Message: "row-level security policy"}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isRowSecurityRefusal(tc.err); got != tc.want {
				t.Errorf("isRowSecurityRefusal = %v, want %v", got, tc.want)
			}
		})
	}
}
