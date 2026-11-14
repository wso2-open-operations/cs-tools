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
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
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

// A lookup that hangs is cut off after its own short deadline: no icon, and the
// alert is not held up.
func TestStaffLookup_ASlowLookupIsCutOffAndShowsNoIcon(t *testing.T) {
	l := NewStaffLookup(blockingSearcher{}, time.Minute)
	start := time.Now()
	if l.IsInternal(context.Background(), "a@wso2.com") {
		t.Error("a lookup that never answered must not show the icon")
	}
	if time.Since(start) > staffLookupTimeout+time.Second {
		t.Errorf("the lookup waited %v, want about %v at most", time.Since(start), staffLookupTimeout)
	}
}

// blockingSearcher answers only when its context ends.
type blockingSearcher struct{}

func (blockingSearcher) SearchUsersByEmail(ctx context.Context, _ []string) ([]entity.UserRoleInfo, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// countingStaffLookup answers "not internal" and counts how often it was asked,
// like a lookup that is failing.
type countingStaffLookup struct{ asked int }

func (c *countingStaffLookup) IsInternal(context.Context, string) bool {
	c.asked++
	return false
}

// An event for recipients in two groups (customer and CSM portals) asks whether
// the person is staff once, not once per group's render: a slow or failing
// lookup would otherwise make every group wait for it again.
func TestDispatcher_Handle_StaffLookupIsAskedOncePerEventNotOncePerGroup(t *testing.T) {
	links := &mockLinkResolver{
		linkFor: func(email string) string {
			if email == "customer@example.com" {
				return "https://customer.example/projects/PROJ-1/support/cases/CASE-1"
			}
			return "https://csm.example/cases/CASE-1"
		},
	}
	events := map[string]string{
		"case.comment_added": `{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Jane Doe","authorEmail":"jane.doe@example.com","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["customer@example.com","staff@example.com"]}}`,
		"case.created":       `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Jane Doe","reporterEmail":"jane.doe@example.com","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["customer@example.com","staff@example.com"]}}`,
	}
	for name, raw := range events {
		t.Run(name, func(t *testing.T) {
			mock := &mockEmailSender{}
			lookup := &countingStaffLookup{}
			d := NewDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{}, links, true, false, nil, true, "", nil).WithStaffLookup(lookup)
			if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(raw)}); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if len(mock.calls) != 2 {
				t.Fatalf("emails = %d, want 2 (one per group)", len(mock.calls))
			}
			if lookup.asked != 1 {
				t.Errorf("staff lookup asked %d times, want 1 for the event", lookup.asked)
			}
		})
	}
}
