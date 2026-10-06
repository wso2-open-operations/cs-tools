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
	"errors"
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// recordingUsersByIDsRepo wraps stubUserRepo with a GetUsersByIDs that records
// what userService forwarded to it. Postgres rejects a non-UUID bound against
// the UUID "user".id column for the whole query, so what reaches the
// repository is exactly what these tests need to pin.
type recordingUsersByIDsRepo struct {
	stubUserRepo
	called bool
	gotIDs []string
	users  []domain.User
	err    error
}

func (r *recordingUsersByIDsRepo) GetUsersByIDs(_ context.Context, ids []string) ([]domain.User, error) {
	r.called = true
	r.gotIDs = ids
	return r.users, r.err
}

const (
	usersByIDsFirstID  = "11111111-1111-1111-1111-111111111111"
	usersByIDsSecondID = "22222222-2222-2222-2222-222222222222"
)

// TestUserService_GetUsersByIDs_SkipsNonUUIDIDs proves ids that cannot be a
// "user".id are dropped before the query, so one free-text value in the batch
// (a KB article's updated_by can be an email address or a system name) does
// not fail the lookup of every valid id beside it.
func TestUserService_GetUsersByIDs_SkipsNonUUIDIDs(t *testing.T) {
	repo := &recordingUsersByIDsRepo{
		users: []domain.User{{ID: usersByIDsFirstID}, {ID: usersByIDsSecondID}},
	}
	svc := NewUserService(repo)

	resp, err := svc.GetUsersByIDs(context.Background(), []string{
		usersByIDsFirstID,
		"someone@example.com", // an email, as a free-text updated_by
		"",                    // an absent author id
		"system",              // a bare name
		"not-a-uuid-at-all",
		"33333333-3333-3333-3333-33333333333",  // one character short
		"AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE", // upper case is still a UUID
	})
	if err != nil {
		t.Fatalf("GetUsersByIDs returned error: %v", err)
	}

	want := []string{usersByIDsFirstID, "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"}
	if !reflect.DeepEqual(repo.gotIDs, want) {
		t.Errorf("repository received ids %q, want only the well-formed ones in order: %q", repo.gotIDs, want)
	}
	if len(resp.Users) != 2 {
		t.Errorf("Users = %+v, want the repository's two users passed through", resp.Users)
	}
}

// TestUserService_GetUsersByIDs_NoValidIDsSkipsRepository proves a batch with
// nothing that could match answers with an empty (non-nil) list without
// querying at all.
func TestUserService_GetUsersByIDs_NoValidIDsSkipsRepository(t *testing.T) {
	for name, ids := range map[string][]string{
		"nil":            nil,
		"empty":          {},
		"only empty id":  {""},
		"only free text": {"someone@example.com", "system"},
		"only malformed": {"11111111-1111-1111-1111", "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &recordingUsersByIDsRepo{}
			svc := NewUserService(repo)

			resp, err := svc.GetUsersByIDs(context.Background(), ids)
			if err != nil {
				t.Fatalf("GetUsersByIDs returned error: %v", err)
			}
			if repo.called {
				t.Errorf("repository was queried with %q, want no query when no id could match", repo.gotIDs)
			}
			if resp.Users == nil || len(resp.Users) != 0 {
				t.Errorf("Users = %#v, want an empty non-nil list", resp.Users)
			}
		})
	}
}

// TestUserService_GetUsersByIDs_PropagatesRepoError proves a genuine
// repository failure is still returned, not swallowed by the filtering.
func TestUserService_GetUsersByIDs_PropagatesRepoError(t *testing.T) {
	wantErr := errors.New("connection reset")
	repo := &recordingUsersByIDsRepo{err: wantErr}
	svc := NewUserService(repo)

	_, err := svc.GetUsersByIDs(context.Background(), []string{usersByIDsFirstID})
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetUsersByIDs error = %v, want %v", err, wantErr)
	}
}
