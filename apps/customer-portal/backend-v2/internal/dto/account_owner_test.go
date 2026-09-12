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

package dto

import (
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestAccountOwner_ResolvesFromAccountManager is the regression guard for the
// portal's account `owner` field silently going null.
//
// entity-service's unified account view renamed `owner` -> `accountManager`.
// This backend kept decoding only `owner`, so the field was absent from every
// account response under both data sources -- invisible, because `owner` is
// optional and an absent optional field looks exactly like an account with no
// owner.
func TestAccountOwner_ResolvesFromAccountManager(t *testing.T) {
	for name, tc := range map[string]struct {
		body     string
		wantID   string
		wantName string
	}{
		"current shape (accountManager)": {
			`{"id":"a-1","accountManager":{"id":"u-1","name":"Jane Doe","email":"jane.doe@example.com"}}`,
			"u-1", "Jane Doe",
		},
		"legacy shape (owner) still honoured": {
			`{"id":"a-1","owner":{"id":"u-2","name":"John Smith"}}`,
			"u-2", "John Smith",
		},
		"owner wins when a source emits both": {
			`{"id":"a-1","owner":{"id":"u-2","name":"John Smith"},"accountManager":{"id":"u-1","name":"Jane Doe"}}`,
			"u-2", "John Smith",
		},
	} {
		var detail entity.AccountDetail
		if err := json.Unmarshal([]byte(tc.body), &detail); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := MapAccountDetails(detail)
		if got.Owner == nil {
			t.Errorf("%s: owner is nil -- the account detail page renders no owner", name)
			continue
		}
		if got.Owner.ID != tc.wantID || got.Owner.Name != tc.wantName {
			t.Errorf("%s: owner = %+v, want {ID:%s Name:%s}", name, *got.Owner, tc.wantID, tc.wantName)
		}
	}
}

// TestAccountOwner_AbsentStaysNil keeps the fallback from inventing an owner
// where the upstream genuinely reports none.
func TestAccountOwner_AbsentStaysNil(t *testing.T) {
	var detail entity.AccountDetail
	if err := json.Unmarshal([]byte(`{"id":"a-1"}`), &detail); err != nil {
		t.Fatal(err)
	}
	if got := MapAccountDetails(detail); got.Owner != nil {
		t.Errorf("owner = %+v, want nil when neither key is present", got.Owner)
	}
}
