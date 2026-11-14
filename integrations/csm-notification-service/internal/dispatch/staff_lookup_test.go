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

package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
)

type fakeUserSearcher struct {
	users []entity.UserRoleInfo
	err   error
	calls int
}

func (f *fakeUserSearcher) SearchUsersByEmail(_ context.Context, _ []string) ([]entity.UserRoleInfo, error) {
	f.calls++
	return f.users, f.err
}

func TestStaffLookup_OnlyAnInternalUserTypeCounts(t *testing.T) {
	cases := []struct {
		name     string
		users    []entity.UserRoleInfo
		wantTrue bool
	}{
		{"internal", []entity.UserRoleInfo{{Email: "a@wso2.com", UserType: "internal"}}, true},
		{"customer with a WSO2 address", []entity.UserRoleInfo{{Email: "a@wso2.com", UserType: "external"}}, false},
		{"no such user", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := NewStaffLookup(&fakeUserSearcher{users: c.users}, time.Minute)
			if got := l.IsInternal(context.Background(), "A@wso2.com"); got != c.wantTrue {
				t.Errorf("IsInternal = %v, want %v", got, c.wantTrue)
			}
		})
	}
}

// A failed lookup shows no icon even for an internal user, and is not
// remembered: the next alert asks again.
func TestStaffLookup_FailureMeansNoIconAndIsNotCached(t *testing.T) {
	s := &fakeUserSearcher{err: errors.New("entity-service down")}
	l := NewStaffLookup(s, time.Minute)
	if l.IsInternal(context.Background(), "a@wso2.com") {
		t.Fatal("a failed lookup must not show the icon")
	}
	s.err = nil
	s.users = []entity.UserRoleInfo{{Email: "a@wso2.com", UserType: "internal"}}
	if !l.IsInternal(context.Background(), "a@wso2.com") {
		t.Error("after a failure the next lookup must ask again")
	}
}

func TestStaffLookup_RemembersAnAnswerUntilItExpires(t *testing.T) {
	s := &fakeUserSearcher{users: []entity.UserRoleInfo{{Email: "a@wso2.com", UserType: "internal"}}}
	l := NewStaffLookup(s, time.Minute).(*cachedStaffLookup)
	now := time.Now()
	l.now = func() time.Time { return now }
	l.IsInternal(context.Background(), "a@wso2.com")
	l.IsInternal(context.Background(), "a@wso2.com")
	if s.calls != 1 {
		t.Errorf("calls = %d, want 1 within the ttl", s.calls)
	}
	l.now = func() time.Time { return now.Add(2 * time.Minute) }
	l.IsInternal(context.Background(), "a@wso2.com")
	if s.calls != 2 {
		t.Errorf("calls = %d, want 2 after the ttl", s.calls)
	}
}

func TestStaffLookup_NoLookupConfiguredShowsNoIcon(t *testing.T) {
	d := &Dispatcher{}
	if d.isStaff(context.Background(), "a@wso2.com") {
		t.Error("no lookup configured must mean no icon")
	}
}
