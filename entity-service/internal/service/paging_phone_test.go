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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// People whose numbers the tests set, one per kind of team.
const (
	uidVega     = "10000000-0000-0000-0000-000000000001"
	uidApollo   = "10000000-0000-0000-0000-000000000002"
	uidAsgardeo = "10000000-0000-0000-0000-000000000003"
	uidAmericas = "10000000-0000-0000-0000-000000000004"
	uidNoTeam   = "10000000-0000-0000-0000-000000000005"
	uidUnknown  = "10000000-0000-0000-0000-0000000000ff"
)

// stubContacts is paging_contact in memory, with the same rules the SQL has.
type stubContacts struct {
	users map[string]repository.PagingUser
	teams map[string][]repository.PagingTeamRef
	rows  map[string]repository.PagingContactRow
	// profiles is "user".phone, by user id.
	profiles map[string]string
	// tests is paging_test_call, by user id.
	tests     map[string]repository.PagingTestRow
	lastActor string
}

func newStubContacts() *stubContacts {
	c := &stubContacts{
		users: map[string]repository.PagingUser{},
		teams: map[string][]repository.PagingTeamRef{
			uidVega:     {{TeamKey: "vega", Family: "CRE"}},
			uidApollo:   {{TeamKey: "apollo", Family: "SRE"}},
			uidAsgardeo: {{TeamKey: "asgardeo", Family: "SME"}},
			uidAmericas: {{TeamKey: "americas", Family: "CRE"}},
		},
		rows:     map[string]repository.PagingContactRow{},
		profiles: map[string]string{},
		tests:    map[string]repository.PagingTestRow{},
	}
	for id, name := range map[string]string{uidVega: "Vic", uidApollo: "Abe", uidAsgardeo: "Ama", uidAmericas: "Ana", uidNoTeam: "Nia"} {
		c.users[id] = repository.PagingUser{ID: id, Email: strings.ToLower(name) + "@wso2.com", Name: name}
	}
	return c
}

func (c *stubContacts) User(_ context.Context, id string) (repository.PagingUser, error) {
	u, ok := c.users[id]
	if !ok {
		return repository.PagingUser{}, &apierror.NotFoundError{Msg: "user not found"}
	}
	return u, nil
}

func (c *stubContacts) UserTeams(_ context.Context, ids []string) (map[string][]repository.PagingTeamRef, error) {
	out := map[string][]repository.PagingTeamRef{}
	for _, id := range ids {
		if t, ok := c.teams[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

func (c *stubContacts) Get(_ context.Context, id string) (*repository.PagingContactRow, error) {
	r, ok := c.rows[id]
	if !ok {
		return nil, nil
	}
	return &r, nil
}

func (c *stubContacts) ByUserIDs(_ context.Context, ids []string) (map[string]repository.PagingContactRow, error) {
	out := map[string]repository.PagingContactRow{}
	for _, id := range ids {
		if r, ok := c.rows[id]; ok {
			out[id] = r
		}
	}
	return out, nil
}

func (c *stubContacts) DialNumbersByEmails(_ context.Context, emails []string) ([]repository.PagingDialRow, error) {
	out := []repository.PagingDialRow{}
	for _, e := range emails {
		for id, u := range c.users {
			if !strings.EqualFold(u.Email, strings.TrimSpace(e)) {
				continue
			}
			d := repository.PagingDialRow{UserID: id, Email: u.Email, ProfilePhone: strings.TrimSpace(c.profiles[id])}
			if r, ok := c.rows[id]; ok {
				d.Paging = &r
			}
			if d.ProfilePhone != "" || d.Paging != nil {
				out = append(out, d)
			}
		}
	}
	return out, nil
}

func (c *stubContacts) ProfilePhones(_ context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		if p := strings.TrimSpace(c.profiles[id]); e164.MatchString(p) {
			out[id] = p
		}
	}
	return out, nil
}

func (c *stubContacts) ProfileTests(_ context.Context, ids []string) (map[string]repository.PagingTestRow, error) {
	out := map[string]repository.PagingTestRow{}
	for _, id := range ids {
		if t, ok := c.tests[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

func (c *stubContacts) MarkProfileTestPending(_ context.Context, actor, id, phone string, at, notBefore time.Time) (bool, error) {
	if t, ok := c.tests[id]; ok && t.LastTestAt.After(notBefore) {
		return false, nil
	}
	if r, ok := c.rows[id]; ok && r.LastTestAt != nil && r.LastTestAt.After(notBefore) {
		return false, nil
	}
	c.lastActor = actor
	c.tests[id] = repository.PagingTestRow{UserID: id, Phone: phone, LastTestAt: at, LastTestStatus: "pending"}
	return true, nil
}

func (c *stubContacts) Upsert(_ context.Context, actor, id, phone string) (repository.PagingContactRow, error) {
	c.lastActor = actor
	u := c.users[id]
	r := repository.PagingContactRow{UserID: id, Email: u.Email, Name: u.Name, Phone: phone, SetBy: actor, SetAt: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
	if old, ok := c.rows[id]; ok && old.Phone == phone {
		r.LastTestAt, r.LastTestStatus = old.LastTestAt, old.LastTestStatus
	}
	c.rows[id] = r
	return r, nil
}

func (c *stubContacts) Delete(_ context.Context, actor, id string) error {
	c.lastActor = actor
	delete(c.rows, id)
	return nil
}

func (c *stubContacts) MarkTestPending(_ context.Context, actor, id string, at, notBefore time.Time) (repository.PagingContactRow, bool, error) {
	r, ok := c.rows[id]
	if !ok || (r.LastTestAt != nil && r.LastTestAt.After(notBefore)) {
		return repository.PagingContactRow{}, false, nil
	}
	if t, ok := c.tests[id]; ok && t.LastTestAt.After(notBefore) {
		return repository.PagingContactRow{}, false, nil
	}
	c.lastActor = actor
	pending := "pending"
	r.LastTestAt, r.LastTestStatus = &at, &pending
	c.rows[id] = r
	return r, true, nil
}

func (c *stubContacts) RecordTestResult(_ context.Context, actor, id, status string, at time.Time) (bool, error) {
	if t, ok := c.tests[id]; ok && t.LastTestStatus == "pending" {
		r, rok := c.rows[id]
		newerPaging := rok && r.LastTestStatus != nil && *r.LastTestStatus == "pending" && r.LastTestAt.After(t.LastTestAt)
		if !newerPaging {
			c.lastActor = actor
			t.LastTestAt, t.LastTestStatus = at, status
			c.tests[id] = t
			return true, nil
		}
	}
	r, ok := c.rows[id]
	if !ok || r.LastTestStatus == nil || *r.LastTestStatus != "pending" {
		return false, nil
	}
	c.lastActor = actor
	r.LastTestAt, r.LastTestStatus = &at, &status
	c.rows[id] = r
	return true, nil
}

// phoneFixture is the paging chain fixture with the SRE and SME rota admins
// added, over in-memory contacts.
func phoneFixture() (*stubPagingRepo, *stubContacts) {
	repo := pagingFixture()
	repo.callers["sreadmin@wso2.com"] = repository.PagingCaller{GlobalRoles: []string{"sre_rota_admin"}}
	repo.callers["smeadmin@wso2.com"] = repository.PagingCaller{GlobalRoles: []string{"sme_rota_admin"}}
	repo.callers["apollolead@example.com"] = repository.PagingCaller{Memberships: []repository.PagingCallerMembership{{TeamKey: "apollo", Role: "lead"}}}
	return repo, newStubContacts()
}

func phoneService(repo *stubPagingRepo, contacts *stubContacts, pub EventPublisherService, now time.Time) *pagingChainService {
	svc := NewPagingChainService(repo, contacts, alwaysUnrestrictedAccess{}, nil, pub).(*pagingChainService)
	svc.now = func() time.Time { return now }
	return svc
}

// The agreed rule for who may set a person's paging number (P3).
func TestSetPagingPhone_PermissionTable(t *testing.T) {
	for _, tc := range []struct {
		caller, target string
		allow          bool
	}{
		{"vegalead@example.com", uidVega, true},
		{"vegalead@example.com", uidApollo, false},
		{"apollolead@example.com", uidApollo, true},
		{"amhead@wso2.com", uidAmericas, true}, // americas_team_lead leads the team
		{"amhead@wso2.com", uidVega, false},
		{"rota@wso2.com", uidVega, true}, // cre_rota_admin, CRE team
		{"rota@wso2.com", uidApollo, false},
		{"rota@wso2.com", uidNoTeam, false},
		{"sreadmin@wso2.com", uidApollo, true},
		{"sreadmin@wso2.com", uidVega, false},
		{"smeadmin@wso2.com", uidAsgardeo, true},
		{"smeadmin@wso2.com", uidApollo, false},
		{"crehead@wso2.com", uidApollo, true}, // a head, whatever the team
		{"crehead@wso2.com", uidNoTeam, true},
		{"admin@wso2.com", uidNoTeam, true},
		{"eng@wso2.com", uidVega, false},
	} {
		t.Run(tc.caller+"->"+tc.target, func(t *testing.T) {
			repo, contacts := phoneFixture()
			got, err := phoneService(repo, contacts, nil, time.Now()).SetPagingPhone(pagingCaller(tc.caller),
				domain.SetPagingPhoneRequest{UserID: tc.target, Phone: " +94771234123 "})
			var forbidden *apierror.ForbiddenError
			switch {
			case tc.allow && err != nil:
				t.Fatalf("want allowed, got %v", err)
			case tc.allow && (got.Phone != "+94771234123" || contacts.lastActor != tc.caller):
				t.Fatalf("stored %+v by %q", got, contacts.lastActor)
			case !tc.allow && !errors.As(err, &forbidden):
				t.Fatalf("want ForbiddenError, got %v", err)
			case !tc.allow && len(contacts.rows) != 0:
				t.Fatal("refused, but a number was stored")
			}
		})
	}
}

func TestSetPagingPhone_Refusals(t *testing.T) {
	repo, contacts := phoneFixture()
	svc := phoneService(repo, contacts, nil, time.Now())
	for _, phone := range []string{"0771234567", "+0771234567", "+94 771234567", "+123456", "+1234567890123456", "94771234567", ""} {
		_, err := svc.SetPagingPhone(pagingCaller("rota@wso2.com"), domain.SetPagingPhoneRequest{UserID: uidVega, Phone: phone})
		var invalid *apierror.ValidationError
		if !errors.As(err, &invalid) {
			t.Errorf("phone %q: want ValidationError, got %v", phone, err)
		}
	}
	_, err := svc.SetPagingPhone(pagingCaller("rota@wso2.com"), domain.SetPagingPhoneRequest{UserID: "vic", Phone: "+94771234123"})
	var invalid *apierror.ValidationError
	if !errors.As(err, &invalid) {
		t.Errorf("non-uuid user: %v", err)
	}
	_, err = svc.SetPagingPhone(pagingCaller("rota@wso2.com"), domain.SetPagingPhoneRequest{UserID: uidUnknown, Phone: "+94771234123"})
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("unknown user: %v", err)
	}
	// A service credential reads but never writes.
	_, err = svc.SetPagingPhone(context.Background(), domain.SetPagingPhoneRequest{UserID: uidVega, Phone: "+94771234123"})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) || len(contacts.rows) != 0 {
		t.Errorf("service credential: %v, rows %d", err, len(contacts.rows))
	}
}

func TestDeletePagingPhone(t *testing.T) {
	repo, contacts := phoneFixture()
	svc := phoneService(repo, contacts, nil, time.Now())
	_, _ = contacts.Upsert(context.Background(), "x", uidVega, "+94771234123")
	var forbidden *apierror.ForbiddenError
	if err := svc.DeletePagingPhone(pagingCaller("eng@wso2.com"), uidVega); !errors.As(err, &forbidden) || len(contacts.rows) != 1 {
		t.Fatalf("engineer delete: %v", err)
	}
	if err := svc.DeletePagingPhone(pagingCaller("vegalead@example.com"), uidVega); err != nil || len(contacts.rows) != 0 || contacts.lastActor != "vegalead@example.com" {
		t.Fatalf("lead delete: %v, rows %d, actor %q", err, len(contacts.rows), contacts.lastActor)
	}
	if err := svc.DeletePagingPhone(pagingCaller("vegalead@example.com"), uidVega); err != nil {
		t.Errorf("deleting nothing is not an error: %v", err)
	}
}

func TestMaskPhone(t *testing.T) {
	for phone, want := range map[string]string{
		"+94771234123":   "+94•••••123", // Sri Lanka, two-digit code
		"+14155550123":   "+1•••••123",  // NANP, one digit
		"+79161234567":   "+7•••••567",
		"+447911123456":  "+44•••••456",
		"+353871234567":  "+353•••••567", // three-digit code
		"+8801712345678": "+880•••••678",
		"+1234567":       "+1•••••567",
		"+353123":        "+•••••", // too short to show both ends
	} {
		if got := maskPhone(phone); got != want {
			t.Errorf("maskPhone(%s) = %s, want %s", phone, got, want)
		}
	}
}

func TestTestCallCooldown(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	var tooMany *apierror.TooManyRequestsError
	for _, d := range []time.Duration{0, time.Minute, 119 * time.Second} {
		if err := testCallCooldown(now, at(d)); !errors.As(err, &tooMany) {
			t.Errorf("last test %v ago: %v, want 429", d, err)
		}
		// Either number's test counts.
		if err := testCallCooldown(now, at(time.Hour), at(d)); !errors.As(err, &tooMany) {
			t.Errorf("paging test %v ago: %v, want 429", d, err)
		}
	}
	for _, tests := range [][]*time.Time{nil, {nil}, {nil, nil}, {at(2 * time.Minute)}, {at(time.Hour), nil}} {
		if err := testCallCooldown(now, tests...); err != nil {
			t.Errorf("%v: %v, want allowed", tests, err)
		}
	}
}

func TestRequestTestCall(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	repo, contacts := phoneFixture()
	pub := &mockEventPublisher{}
	svc := phoneService(repo, contacts, pub, now)

	var conflict *apierror.ConflictError
	if _, err := svc.RequestTestCall(pagingCaller("rota@wso2.com"), uidVega); !errors.As(err, &conflict) || len(pub.calls) != 0 {
		t.Fatalf("no number: %v", err)
	}
	_, _ = contacts.Upsert(context.Background(), "x", uidVega, "+94771234123")
	var forbidden *apierror.ForbiddenError
	for _, caller := range []string{"eng@wso2.com", "vegalead@example.com", "crehead@wso2.com"} {
		if _, err := svc.RequestTestCall(pagingCaller(caller), uidVega); !errors.As(err, &forbidden) {
			t.Fatalf("%s: %v, want 403", caller, err)
		}
	}

	resp, err := svc.RequestTestCall(pagingCaller("rota@wso2.com"), uidVega)
	if err != nil || resp.LastTestStatus != "pending" {
		t.Fatalf("test call: %+v %v", resp, err)
	}
	row := contacts.rows[uidVega]
	if row.LastTestStatus == nil || *row.LastTestStatus != "pending" || !row.LastTestAt.Equal(now) {
		t.Errorf("stored %+v", row)
	}
	call, ok := findPublishCall(pub.calls, events.TypePagingTestCallRequested)
	if !ok || call.entityID != uidVega {
		t.Fatalf("published %v (entity %q)", publishedTypes(pub.calls), call.entityID)
	}
	dec := json.NewDecoder(bytes.NewReader(call.payload))
	dec.DisallowUnknownFields()
	var p events.PagingTestCallRequestedPayload
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("strict decode %s: %v", call.payload, err)
	}
	want := events.PagingTestCallRequestedPayload{
		UserID: uidVega, Email: "vic@wso2.com", Name: "Vic", Phone: "+94771234123",
		RequestedBy: "rota@wso2.com", RequestedAt: "2026-10-08T10:00:00Z",
	}
	if p != want {
		t.Errorf("payload\n got %+v\nwant %+v", p, want)
	}

	// Again within two minutes: 429, nothing published.
	svc.now = func() time.Time { return now.Add(90 * time.Second) }
	var tooMany *apierror.TooManyRequestsError
	if _, err := svc.RequestTestCall(pagingCaller("rota@wso2.com"), uidVega); !errors.As(err, &tooMany) || len(pub.calls) != 1 {
		t.Fatalf("within the cooldown: %v, %d published", err, len(pub.calls))
	}
	svc.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := svc.RequestTestCall(pagingCaller("rota@wso2.com"), uidVega); err != nil || len(pub.calls) != 2 {
		t.Fatalf("after the cooldown: %v, %d published", err, len(pub.calls))
	}
}

func TestRequestTestCall_PublishProblems(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	repo, contacts := phoneFixture()
	_, _ = contacts.Upsert(context.Background(), "x", uidVega, "+94771234123")

	var unavailable *apierror.ServiceUnavailableError
	if _, err := phoneService(repo, contacts, nil, now).RequestTestCall(pagingCaller("rota@wso2.com"), uidVega); !errors.As(err, &unavailable) {
		t.Fatalf("no publisher: %v", err)
	}
	if contacts.rows[uidVega].LastTestStatus != nil {
		t.Fatal("no publisher, yet a test was marked pending")
	}

	failing := &mockEventPublisher{err: errors.New("event hub down")}
	if _, err := phoneService(repo, contacts, failing, now).RequestTestCall(pagingCaller("rota@wso2.com"), uidVega); !errors.As(err, &unavailable) {
		t.Fatalf("publish failed: %v", err)
	}
	if s := contacts.rows[uidVega].LastTestStatus; s == nil || *s != "failed" {
		t.Errorf("a test that was never sent reads %v, want failed", s)
	}
}

func TestListPagingContacts(t *testing.T) {
	repo, contacts := phoneFixture()
	_, _ = contacts.Upsert(context.Background(), "lead@example.com", uidVega, "+94771234123")
	svc := phoneService(repo, contacts, nil, time.Now())

	// A service credential may read: it is what pages.
	resp, err := svc.ListPagingContacts(context.Background(), []string{" VIC@wso2.com ", "abe@wso2.com", ""})
	if err != nil || len(resp.Contacts) != 1 {
		t.Fatalf("%+v %v", resp, err)
	}
	if c := resp.Contacts[0]; c.UserID != uidVega || c.Phone != "+94771234123" || c.SetBy != "lead@example.com" || c.SetAt != "2026-10-08T10:00:00Z" ||
		c.DialPhone != "+94771234123" || c.DialSource != "paging" || c.ProfilePhone != "" {
		t.Errorf("contact %+v", c)
	}
	if resp, err := svc.ListPagingContacts(context.Background(), nil); err != nil || resp.Contacts == nil || len(resp.Contacts) != 0 {
		t.Errorf("no emails: %+v %v", resp, err)
	}
	many := make([]string, 201)
	for i := range many {
		many[i] = fmt.Sprintf("p%d@wso2.com", i)
	}
	var invalid *apierror.ValidationError
	if _, err := svc.ListPagingContacts(context.Background(), many); !errors.As(err, &invalid) {
		t.Errorf("201 emails: %v", err)
	}
	var forbidden *apierror.ForbiddenError
	denied := NewPagingChainService(repo, contacts, deniedAccess{}, nil, nil)
	if _, err := denied.ListPagingContacts(context.Background(), []string{"vic@wso2.com"}); !errors.As(err, &forbidden) {
		t.Errorf("customer: %v", err)
	}
}

// The number to call is the one on the person's own profile when it can be
// called, else the paging-only number; someone with neither is left out.
func TestListPagingContacts_DialNumber(t *testing.T) {
	repo, contacts := phoneFixture()
	ctx := context.Background()
	_, _ = contacts.Upsert(ctx, "lead@example.com", uidVega, "+94771234123")
	_, _ = contacts.Upsert(ctx, "lead@example.com", uidApollo, "+94771234124")
	contacts.profiles[uidVega] = " +94770000001 "   // both: the profile wins
	contacts.profiles[uidApollo] = "077 000 0002"   // not callable: the paging number
	contacts.profiles[uidAsgardeo] = "+94770000003" // profile only
	contacts.profiles[uidNoTeam] = "0770000005"     // neither callable, no paging number
	svc := phoneService(repo, contacts, nil, time.Now())

	resp, err := svc.ListPagingContacts(ctx, []string{"vic@wso2.com", "abe@wso2.com", "ama@wso2.com", "ana@wso2.com", "nia@wso2.com"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.PagingContact{}
	for _, c := range resp.Contacts {
		got[c.UserID] = c
	}
	for id, want := range map[string][3]string{ // dial number, source, paging-only number
		uidVega:     {"+94770000001", "profile", "+94771234123"},
		uidApollo:   {"+94771234124", "paging", "+94771234124"},
		uidAsgardeo: {"+94770000003", "profile", ""},
		uidNoTeam:   {"", "", ""},
	} {
		c, ok := got[id]
		if !ok {
			t.Errorf("%s missing", id)
			continue
		}
		if c.DialPhone != want[0] || c.DialSource != want[1] || c.Phone != want[2] {
			t.Errorf("%s = dial %q (%s), paging %q; want %v", id, c.DialPhone, c.DialSource, c.Phone, want)
		}
	}
	if _, ok := got[uidAmericas]; ok {
		t.Error("a person with neither number must be left out")
	}
	raw, _ := json.Marshal(got[uidAsgardeo])
	for _, absent := range []string{`"setBy"`, `"setAt"`, `"lastTestStatus"`} {
		if strings.Contains(string(raw), absent) {
			t.Errorf("profile-only contact carries %s: %s", absent, raw)
		}
	}
}

func TestRecordTestResult(t *testing.T) {
	repo, contacts := phoneFixture()
	_, _ = contacts.Upsert(context.Background(), "x", uidVega, "+94771234123")
	_, _, _ = contacts.MarkTestPending(context.Background(), "x", uidVega, time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC), time.Date(2026, 10, 8, 9, 58, 0, 0, time.UTC))
	svc := phoneService(repo, contacts, nil, time.Now())
	service := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: "csm-notification-service"})
	ok := domain.PagingTestResultRequest{UserID: uidVega, Status: "no-answer", TestedAt: "2026-10-08T10:01:30Z"}

	var forbidden *apierror.ForbiddenError
	if err := svc.RecordTestResult(pagingCaller("vegalead@example.com"), ok); !errors.As(err, &forbidden) {
		t.Errorf("a person reported a result: %v", err)
	}
	var invalid *apierror.ValidationError
	for _, bad := range []domain.PagingTestResultRequest{
		{UserID: uidVega, Status: "pending", TestedAt: ok.TestedAt},
		{UserID: uidVega, Status: "answered", TestedAt: ok.TestedAt},
		{UserID: uidVega, Status: "busy", TestedAt: "yesterday"},
		{UserID: "vic", Status: "busy", TestedAt: ok.TestedAt},
	} {
		if err := svc.RecordTestResult(service, bad); !errors.As(err, &invalid) {
			t.Errorf("%+v: %v, want ValidationError", bad, err)
		}
	}
	var notFound *apierror.NotFoundError
	if err := svc.RecordTestResult(service, domain.PagingTestResultRequest{UserID: uidApollo, Status: "busy", TestedAt: ok.TestedAt}); !errors.As(err, &notFound) {
		t.Errorf("no number: %v", err)
	}
	if err := svc.RecordTestResult(service, ok); err != nil {
		t.Fatal(err)
	}
	row := contacts.rows[uidVega]
	if *row.LastTestStatus != "no-answer" || row.LastTestAt.Format(time.RFC3339) != ok.TestedAt || contacts.lastActor != "service:csm-notification-service" {
		t.Errorf("stored %v at %v by %q", *row.LastTestStatus, row.LastTestAt, contacts.lastActor)
	}
	// A repeated result finds no pending test and changes nothing.
	again := domain.PagingTestResultRequest{UserID: uidVega, Status: "completed", TestedAt: "2026-10-08T10:05:00Z"}
	if err := svc.RecordTestResult(service, again); !errors.As(err, &notFound) {
		t.Errorf("repeated result: %v, want NotFoundError", err)
	}
	if row := contacts.rows[uidVega]; *row.LastTestStatus != "no-answer" {
		t.Errorf("a repeated result overwrote the last one: %v", *row.LastTestStatus)
	}
}

// The chain shows a number in full only to someone who may edit it.
func TestGetPagingChain_PagingPhones(t *testing.T) {
	repo, contacts := phoneFixture()
	vega := repo.members[idVegaEng]
	vega.UserID = uidVega
	repo.members[idVegaEng] = vega
	am := repo.members[idAmEng]
	am.UserID = uidAmericas
	repo.members[idAmEng] = am
	_, _ = contacts.Upsert(context.Background(), "x", uidVega, "+94771234123")
	_, _ = contacts.Upsert(context.Background(), "x", uidAmericas, "+14155550123")
	svc := phoneService(repo, contacts, nil, time.Now())

	view := func(ctx context.Context) map[string]domain.PagingChainMember {
		resp, err := svc.GetPagingChain(ctx, "CRE")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]domain.PagingChainMember{}
		for _, m := range resp.Members {
			out[m.MembershipID] = m
		}
		return out
	}

	got := view(pagingCaller("vegalead@example.com"))
	if m := got[idVegaEng]; !m.CanEditPhone || m.PagingPhone == nil || m.PagingPhone.Phone != "+94771234123" || m.PagingPhone.Masked != "+94•••••123" {
		t.Errorf("vega lead on vega: %+v %+v", m, m.PagingPhone)
	}
	if m := got[idAmEng]; m.CanEditPhone || m.PagingPhone == nil || m.PagingPhone.Phone != "" || m.PagingPhone.Masked != "+1•••••123" {
		t.Errorf("vega lead on americas: %+v %+v", m, m.PagingPhone)
	}
	if m := got[idVegaLead]; m.PagingPhone != nil {
		t.Errorf("no number must be null, got %+v", m.PagingPhone)
	}
	raw, _ := json.Marshal(got[idVegaLead])
	if !strings.Contains(string(raw), `"pagingPhone":null`) || !strings.Contains(string(raw), `"canEditPhone":`) {
		t.Errorf("wire shape %s", raw)
	}

	// A service credential sees masks only.
	for _, m := range view(context.Background()) {
		if m.CanEditPhone || (m.PagingPhone != nil && m.PagingPhone.Phone != "") {
			t.Errorf("service credential got %+v %+v", m, m.PagingPhone)
		}
	}

	// Whether each member has a callable number on their own profile.
	contacts.profiles[uidVega] = "+94770000001"
	contacts.profiles[uidAmericas] = "not a number"
	got = view(pagingCaller("vegalead@example.com"))
	for id, want := range map[string]bool{idVegaEng: true, idAmEng: false, idVegaLead: false} {
		if m := got[id]; m.HasProfilePhone == nil || *m.HasProfilePhone != want {
			t.Errorf("%s hasProfilePhone = %v, want %v", id, m.HasProfilePhone, want)
		}
	}
}

// Who may place a test call: a portal admin, or the rota admin of a family
// one of the person's teams is in. Leads and heads may set a number, not ring it.
func TestRequestTestCall_PermissionTable(t *testing.T) {
	for _, tc := range []struct {
		caller, target string
		allow          bool
	}{
		{"admin@wso2.com", uidVega, true},
		{"admin@wso2.com", uidApollo, true},
		{"admin@wso2.com", uidNoTeam, true},
		{"rota@wso2.com", uidVega, true}, // cre_rota_admin, CRE team
		{"rota@wso2.com", uidAmericas, true},
		{"rota@wso2.com", uidApollo, false},
		{"rota@wso2.com", uidNoTeam, false},
		{"sreadmin@wso2.com", uidApollo, true},
		{"sreadmin@wso2.com", uidVega, false},
		{"smeadmin@wso2.com", uidAsgardeo, true},
		{"smeadmin@wso2.com", uidVega, false},
		{"vegalead@example.com", uidVega, false},
		{"apollolead@example.com", uidApollo, false},
		{"amhead@wso2.com", uidAmericas, false},
		{"crehead@wso2.com", uidVega, false},
		{"eng@wso2.com", uidVega, false},
	} {
		t.Run(tc.caller+"->"+tc.target, func(t *testing.T) {
			repo, contacts := phoneFixture()
			contacts.profiles[tc.target] = "+94770000009"
			pub := &mockEventPublisher{}
			_, err := phoneService(repo, contacts, pub, time.Now()).RequestTestCall(pagingCaller(tc.caller), tc.target)
			var forbidden *apierror.ForbiddenError
			switch {
			case tc.allow && (err != nil || len(pub.calls) != 1):
				t.Fatalf("want allowed, got %v (%d published)", err, len(pub.calls))
			case !tc.allow && (!errors.As(err, &forbidden) || len(pub.calls) != 0 || len(contacts.tests) != 0):
				t.Fatalf("want ForbiddenError and nothing written, got %v (%d published)", err, len(pub.calls))
			}
		})
	}
}

// A person with a callable profile number is rung on it -- the number Case
// Alerts dials first -- and the test is kept apart from the paging-only one.
func TestRequestTestCall_ProfileNumber(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	repo, contacts := phoneFixture()
	pub := &mockEventPublisher{}
	svc := phoneService(repo, contacts, pub, now)
	service := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: "csm-notification-service"})

	contacts.profiles[uidVega] = " +94770000001 "
	_, _ = contacts.Upsert(context.Background(), "x", uidVega, "+94771234123")
	if _, err := svc.RequestTestCall(pagingCaller("rota@wso2.com"), uidVega); err != nil {
		t.Fatal(err)
	}
	call, ok := findPublishCall(pub.calls, events.TypePagingTestCallRequested)
	var p events.PagingTestCallRequestedPayload
	if !ok || json.Unmarshal(call.payload, &p) != nil || p.Phone != "+94770000001" {
		t.Fatalf("rang %q, want the profile number", p.Phone)
	}
	if tr := contacts.tests[uidVega]; tr.Phone != "+94770000001" || tr.LastTestStatus != "pending" || !tr.LastTestAt.Equal(now) || contacts.lastActor != "rota@wso2.com" {
		t.Errorf("profile test %+v by %q", tr, contacts.lastActor)
	}
	if contacts.rows[uidVega].LastTestStatus != nil {
		t.Error("a profile test was recorded on the paging-only number")
	}

	// The cooldown spans both numbers.
	svc.now = func() time.Time { return now.Add(time.Minute) }
	var tooMany *apierror.TooManyRequestsError
	if _, err := svc.RequestTestCall(pagingCaller("rota@wso2.com"), uidVega); !errors.As(err, &tooMany) {
		t.Fatalf("within the cooldown: %v", err)
	}

	// The outcome lands on the pending profile test.
	if err := svc.RecordTestResult(service, domain.PagingTestResultRequest{UserID: uidVega, Status: "completed", TestedAt: "2026-10-08T10:01:10Z"}); err != nil {
		t.Fatal(err)
	}
	if tr := contacts.tests[uidVega]; tr.LastTestStatus != "completed" || tr.LastTestAt.Format(time.RFC3339) != "2026-10-08T10:01:10Z" {
		t.Errorf("profile test after the result %+v", tr)
	}
	if contacts.rows[uidVega].LastTestStatus != nil {
		t.Error("the result was recorded on the paging-only number")
	}

	// No number anywhere: 409, even for an admin.
	var conflict *apierror.ConflictError
	if _, err := svc.RequestTestCall(pagingCaller("admin@wso2.com"), uidApollo); !errors.As(err, &conflict) {
		t.Errorf("no number: %v, want 409", err)
	}
	// A profile number only: rung without a paging_contact row.
	contacts.profiles[uidAsgardeo] = "+94770000003"
	if _, err := svc.RequestTestCall(pagingCaller("smeadmin@wso2.com"), uidAsgardeo); err != nil {
		t.Errorf("profile only: %v", err)
	}
}

// The chain says who may test, and what a test would ring, only to them.
func TestGetPagingChain_TestCall(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	repo, contacts := phoneFixture()
	vega := repo.members[idVegaEng]
	vega.UserID = uidVega
	repo.members[idVegaEng] = vega
	am := repo.members[idAmEng]
	am.UserID = uidAmericas
	repo.members[idAmEng] = am
	contacts.profiles[uidVega] = "+94770000001"
	_, _ = contacts.Upsert(context.Background(), "x", uidAmericas, "+14155550123")
	contacts.tests[uidVega] = repository.PagingTestRow{UserID: uidVega, Phone: "+94770000001", LastTestAt: now, LastTestStatus: "no-answer"}
	svc := phoneService(repo, contacts, nil, now)

	view := func(caller string) map[string]domain.PagingChainMember {
		resp, err := svc.GetPagingChain(pagingCaller(caller), "CRE")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]domain.PagingChainMember{}
		for _, m := range resp.Members {
			out[m.MembershipID] = m
		}
		return out
	}

	got := view("rota@wso2.com")
	if m := got[idVegaEng]; !m.CanTestCall || m.TestCall == nil || m.TestCall.Source != "profile" || m.TestCall.Masked != "+94•••••001" ||
		m.TestCall.LastTestStatus == nil || *m.TestCall.LastTestStatus != "no-answer" || *m.TestCall.LastTestAt != "2026-10-08T10:00:00Z" {
		t.Errorf("profile number: %+v %+v", m, m.TestCall)
	}
	if m := got[idAmEng]; !m.CanTestCall || m.TestCall == nil || m.TestCall.Source != "paging" || m.TestCall.Masked != "+1•••••123" || m.TestCall.LastTestStatus != nil {
		t.Errorf("paging-only number: %+v %+v", m, m.TestCall)
	}
	// A portal admin may test anyone; a person with no number has no testCall.
	if m := view("admin@wso2.com")[idVegaLead]; !m.CanTestCall || m.TestCall != nil {
		t.Errorf("no number: canTestCall %v, testCall %+v", m.CanTestCall, m.TestCall)
	}

	// A team lead may edit a paging number but neither test it nor see testCall.
	for id, m := range view("vegalead@example.com") {
		if m.CanTestCall || m.TestCall != nil {
			t.Errorf("lead sees %s: canTestCall %v, testCall %+v", id, m.CanTestCall, m.TestCall)
		}
	}
	raw, _ := json.Marshal(view("vegalead@example.com")[idVegaEng])
	if !strings.Contains(string(raw), `"canTestCall":false`) || strings.Contains(string(raw), `"testCall"`) {
		t.Errorf("wire shape %s", raw)
	}

	// A test of a profile number that has since changed is not shown.
	contacts.profiles[uidVega] = "+94770000002"
	if m := view("rota@wso2.com")[idVegaEng]; m.TestCall == nil || m.TestCall.Masked != "+94•••••002" || m.TestCall.LastTestStatus != nil {
		t.Errorf("after the number changed: %+v", m.TestCall)
	}
}
