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

package service

import (
	"net/http"
	"testing"
)

// snAccountJSONWithPartner is a minimal ServiceNow account payload with an optional
// "partner"/"primaryPartnerAccountId" fragment appended, so tests can exercise the three
// possible wire states: key entirely absent, key present with an empty reference, and key
// present with a real reference.
func snAccountJSONWithPartner(id, partnerFragment string) string {
	return `{
		"id": "` + id + `",
		"name": "Acme",
		"classification": "enterprise",
		"activationDate": "2026-01-01 00:00:00",
		"hasAgent": false,
		"hasKbReferences": false,
		"createdOn": "2026-01-01 00:00:00",
		"updatedOn": "2026-01-01 00:00:00"` + partnerFragment + `}`
}

// TestSNAccountService_PartnerFields covers IsPartner/HasPrimaryPartner derivation from the
// raw ServiceNow partner/primaryPartnerAccountId fields merged in by the Ballerina layer.
func TestSNAccountService_PartnerFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		fragment          string
		wantIsPartner     *bool
		wantHasPrimaryPtr *bool
	}{
		{
			name:              "keys entirely absent -> both nil",
			fragment:          "",
			wantIsPartner:     nil,
			wantHasPrimaryPtr: nil,
		},
		{
			name:              "partner true, primaryPartnerAccountId present with a real id -> true, true",
			fragment:          `, "partner": true, "primaryPartnerAccountId": {"id": "` + sysid32('e') + `", "name": "Parent Co"}`,
			wantIsPartner:     boolPtr(true),
			wantHasPrimaryPtr: boolPtr(true),
		},
		{
			name:              "partner false, primaryPartnerAccountId absent -> false, nil",
			fragment:          `, "partner": false`,
			wantIsPartner:     boolPtr(false),
			wantHasPrimaryPtr: nil,
		},
		{
			name: "primaryPartnerAccountId present but empty id -> hasPrimaryPartner false",
			// This shape is not currently produced by the Ballerina layer (its own
			// mapping collapses present-but-empty into absent before the wire), but the
			// Go-side derivation must still handle it correctly if that ever changes.
			fragment:          `, "primaryPartnerAccountId": {"id": "", "name": ""}`,
			wantIsPartner:     nil,
			wantHasPrimaryPtr: boolPtr(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mux := http.NewServeMux()
			mux.HandleFunc("/accounts/"+testAccountSysid, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(snAccountJSONWithPartner(testAccountSysid, tt.fragment)))
			})

			client := newTestSNClient(t, mux)
			svc := NewServiceNowAccountService(client)

			got, err := svc.GetAccountByID(contextWithUserIDToken("token"), sysidToUUID(testAccountSysid))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !boolPtrEqual(got.IsPartner, tt.wantIsPartner) {
				t.Errorf("IsPartner = %s, want %s", boolPtrString(got.IsPartner), boolPtrString(tt.wantIsPartner))
			}
			if !boolPtrEqual(got.HasPrimaryPartner, tt.wantHasPrimaryPtr) {
				t.Errorf("HasPrimaryPartner = %s, want %s", boolPtrString(got.HasPrimaryPartner), boolPtrString(tt.wantHasPrimaryPtr))
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func boolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func boolPtrString(b *bool) string {
	if b == nil {
		return "<nil>"
	}
	if *b {
		return "true"
	}
	return "false"
}
