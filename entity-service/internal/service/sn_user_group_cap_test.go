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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestResolveMembershipUserIDs_CapsGroupMembers proves a group filter that
// expands past the upstream id-filter cap is refused with a 400 instead of
// being sent upstream to be silently truncated, while a filter within the
// cap still resolves.
func TestResolveMembershipUserIDs_CapsGroupMembers(t *testing.T) {
	members := 0
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/group-members/search") {
			t.Fatalf("unexpected upstream call %s", r.URL.Path)
		}
		rows := make([]snGroupMembership, 0, members)
		for i := 0; i < members; i++ {
			rows = append(rows, snGroupMembership{UserID: fmt.Sprintf("%032x", i+1), GroupName: "Support"})
		}
		_ = json.NewEncoder(w).Encode(snGroupMembersSearchResponse{Memberships: rows})
	}))
	svc := &snUserService{client: client}
	filters := domain.SearchUsersFilters{GroupNames: []string{"Support"}}

	members = snUserIDFilterLimit
	ids, err := svc.resolveMembershipUserIDs(context.Background(), "", filters)
	if err != nil || len(ids) != snUserIDFilterLimit {
		t.Fatalf("at the cap: got %d ids, err %v; want %d, nil", len(ids), err, snUserIDFilterLimit)
	}

	members = snUserIDFilterLimit + 1
	_, err = svc.resolveMembershipUserIDs(context.Background(), "", filters)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("past the cap: err = %v (%T), want ValidationError", err, err)
	}
}
