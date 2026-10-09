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

package testdb

import "testing"

func TestIsTestDatabase(t *testing.T) {
	cases := map[string]bool{
		"postgres://gid:gid@localhost:5433/gid_test?sslmode=disable": true,
		"postgres://gid:gid@localhost:5433/gid?sslmode=disable":      false,
		"postgres://gid:gid@localhost:5433/gid_testing":              false,
		"postgres://gid:gid@localhost:5433/":                         false,
		"://bad":                                                     false,
		// pgx honours ?dbname= over the URL path, so the effective database
		// is what must be checked, not the path.
		"postgres://gid:gid@localhost:5433/gid_test?dbname=gid": false,
		"postgres://gid:gid@localhost:5433/gid?dbname=gid_test": true,
		"host=localhost port=5433 user=gid dbname=gid_test":     true,
		"host=localhost port=5433 user=gid dbname=gid":          false,
	}
	for dsn, want := range cases {
		if got := isTestDatabase(dsn); got != want {
			t.Errorf("isTestDatabase(%q) = %v, want %v", dsn, got, want)
		}
	}
}
