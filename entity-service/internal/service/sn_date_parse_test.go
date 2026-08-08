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

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestParseSNDateTime_CanonicalFormat pins that the overwhelmingly common
// case -- SN's canonical "YYYY-MM-DD HH:MM:SS" -- keeps parsing exactly as
// before: no behaviour change for the fast path.
func TestParseSNDateTime_CanonicalFormat(t *testing.T) {
	t.Parallel()

	got, err := parseSNDateTime(context.Background(), "test", "updatedOn", "2026-08-05 20:27:43")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 5, 20, 27, 43, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestParseSNDateTime_AlternateFormat pins the fallback that fixes the prod
// incident: SN occasionally renders these fields as "MM-DD-YYYY HH:MM:SS"
// (via GlideRecord.getDisplayValue() upstream) instead of the canonical
// layout. The exact string is taken from a real Choreo prod log line:
// `parse updatedOn "08-05-2026 20:27:43"`.
func TestParseSNDateTime_AlternateFormat(t *testing.T) {
	t.Parallel()

	got, err := parseSNDateTime(context.Background(), "sn update case", "updatedOn", "08-05-2026 20:27:43")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 5, 20, 27, 43, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestParseSNDateTime_MalformedStillErrors pins that a genuinely malformed
// value -- not just a format flip -- still returns an error rather than
// being silently swallowed.
func TestParseSNDateTime_MalformedStillErrors(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "not-a-date", "2026-13-40 99:99:99"} {
		if _, err := parseSNDateTime(context.Background(), "test", "updatedOn", value); err == nil {
			t.Errorf("value %q: got nil error, want a parse error", value)
		}
	}
}

// TestSNCaseService_UpdateCase_AlternateDateFormat reproduces the production
// incident end to end: ServiceNow returns updatedOn in the MM-DD-YYYY
// alternate format on a successful PATCH /cases/{id}. Before the fix this
// caused UpdateCase to return an error -- and the Go service to answer the
// BFF with a 500 -- even though the SN write had already committed. It must
// now succeed and the alternate-format string must parse to the correct
// instant.
func TestSNCaseService_UpdateCase_AlternateDateFormat(t *testing.T) {
	t.Parallel()

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Case updated successfully.",
			"case": {"id": "` + testWLCaseSysid + `", "updatedOn": "08-05-2026 20:27:43", "updatedBy": "engineer@example.com"}
		}`))
	})

	subject := "Updated subject"
	svc := NewServiceNowCaseService(client, nil)
	resp, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{
		ID:      testDeploymentUUID, // any valid UUID; the request id is not asserted here
		Subject: &subject,
	})
	if err != nil {
		t.Fatalf("unexpected error parsing an alternate-format updatedOn: %v", err)
	}
	want := time.Date(2026, 8, 5, 20, 27, 43, 0, time.UTC)
	if !resp.Case.UpdatedOn.Equal(want) {
		t.Errorf("updatedOn = %v, want %v", resp.Case.UpdatedOn, want)
	}
}
