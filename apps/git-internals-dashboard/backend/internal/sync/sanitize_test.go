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

package sync

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
)

// AUDIT-FINDINGS A2: sanitizeSyncError must never leak a raw response-body
// snippet, GraphQL message text, or a wrapped network/DB error chain.
func TestSanitizeSyncErrorClassifications(t *testing.T) {
	secret := "internal-hostname-or-detail-that-must-not-leak"
	apiHTTPErr := github.NewHTTPStatusError(http.StatusBadGateway, 0, secret)

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"github http status", apiHTTPErr, "github http 502"},
		{"github search stage", fmt.Errorf("github search: %w", apiHTTPErr), "github http 502"},
		{"github issue detail stage", fmt.Errorf("github issue detail: %w", fmt.Errorf("boom")), "github issue detail failed"},
		{"ingest stage", fmt.Errorf("ingest: %w", fmt.Errorf("pgx: connection to host 10.0.0.5 failed")), "ingest failed"},
		{"context canceled", context.Canceled, "sync canceled"},
		{"internal config error", fmt.Errorf("repository acme/widgets has no linked project — config sync should have set this"), "repository acme/widgets has no linked project — config sync should have set this"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeSyncError(tc.err)
			if got != tc.want {
				t.Errorf("sanitizeSyncError(%v) = %q, want %q", tc.err, got, tc.want)
			}
			if strings.Contains(got, secret) {
				t.Errorf("sanitized message leaked raw detail: %q", got)
			}
		})
	}
}
