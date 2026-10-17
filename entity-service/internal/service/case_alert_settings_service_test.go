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
	"encoding/base64"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	roomShared = "11111111-1111-1111-1111-111111111111"
	roomApollo = "22222222-2222-2222-2222-222222222222"
	roomCRE    = "44444444-4444-4444-4444-444444444444"
	routeSRE   = "55555555-5555-5555-5555-555555555555"

	// testChatWebhook is shaped like a Google Chat webhook; its key and token
	// are made up.
	testChatWebhook = "https://chat.googleapis.com/v1/spaces/AAAAtest01/messages?key=fake-key&token=fake-token-wxyz"
)

var testCaseAlertKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))

// fakeCaseAlertRepo keeps rooms, secrets and routes in memory and records the
// actor of the last write.
type fakeCaseAlertRepo struct {
	rooms   map[string]*repository.CaseAlertRoomRow
	secrets map[string]repository.CaseAlertSecret
	routes  map[string]repository.CaseAlertRouteRow
	teams   map[string]string // rostered team -> family
	rotas   map[string]string // active rota -> family
	types   map[string]bool   // team types
	shifts  map[string]bool   // active Team Schedule shift codes
	users   map[string]bool   // active users
	values  map[string]repository.CaseAlertValueRow
	summary []byte // applied_summary
	mode    *string
	applied *domain.CaseAlertAppliedReport
	listed  bool
	actor   string
	saved   *repository.CaseAlertRoomWrite
	route   *repository.CaseAlertRouteWrite
}

func newFakeCaseAlertRepo() *fakeCaseAlertRepo {
	return &fakeCaseAlertRepo{
		rooms: map[string]*repository.CaseAlertRoomRow{
			roomShared: {ID: roomShared, Ladder: "SRE", Label: "SRE Case Paging", Enabled: true},
			roomApollo: {ID: roomApollo, Ladder: "SRE", TeamKey: "apollo", Label: "Apollo SRE", Enabled: true, Teamless: true},
			roomCRE:    {ID: roomCRE, Ladder: "CRE", Label: "CRE Case Paging", Enabled: true},
		},
		secrets: map[string]repository.CaseAlertSecret{},
		routes: map[string]repository.CaseAlertRouteRow{
			routeSRE: {ID: routeSRE, Ladder: "SRE", GroupName: "apollo sre group", TeamKey: "apollo", Enabled: true},
		},
		teams:  map[string]string{"apollo": "SRE", "artemis": "SRE", "iaas": "SRE", "vega": "CRE", "atlas": "CRE", "americas": "CRE", "asgardeo": "SME", "u2": "SME"},
		rotas:  map[string]string{"SRE_SAAS": "SRE", "CRE_ABT": "CRE"},
		types:  map[string]bool{"cre-abt": true, "sre-abt": true, "sre-iaas": true},
		shifts: map[string]bool{"CRE_WEEKEND_NIGHT": true},
		users:  map[string]bool{"crehead@wso2.com": true},
		values: map[string]repository.CaseAlertValueRow{},
	}
}

func (f *fakeCaseAlertRepo) Revision(context.Context) (repository.CaseAlertRevisionRow, error) {
	return repository.CaseAlertRevisionRow{Revision: 3, AppliedMode: f.mode, AppliedSummary: f.summary}, nil
}

func (f *fakeCaseAlertRepo) RecordApplied(_ context.Context, by string, rev int64, mode string, summary map[string]any) error {
	f.actor = by
	f.applied = &domain.CaseAlertAppliedReport{Revision: rev, Mode: mode, Summary: summary}
	return nil
}

func (f *fakeCaseAlertRepo) ListRooms(context.Context) ([]repository.CaseAlertRoomRow, error) {
	f.listed = true
	out := []repository.CaseAlertRoomRow{}
	for _, id := range []string{roomCRE, roomShared, roomApollo} {
		if r, ok := f.rooms[id]; ok {
			c := *r
			_, c.HasSecret = f.secrets[id]
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeCaseAlertRepo) GetRoom(_ context.Context, id string) (*repository.CaseAlertRoomRow, error) {
	r, ok := f.rooms[id]
	if !ok {
		return nil, nil
	}
	c := *r
	_, c.HasSecret = f.secrets[id]
	return &c, nil
}

func (f *fakeCaseAlertRepo) SaveRoom(_ context.Context, actor string, w repository.CaseAlertRoomWrite) (repository.CaseAlertRoomRow, error) {
	f.actor, f.saved = actor, &w
	return repository.CaseAlertRoomRow{ID: roomShared, Ladder: w.Ladder, TeamKey: w.TeamKey, Label: w.Label, Enabled: w.Enabled, Teamless: w.Teamless, UpdatedBy: actor}, nil
}

func (f *fakeCaseAlertRepo) DeleteRoom(_ context.Context, actor, id string) (bool, error) {
	f.actor = actor
	_, ok := f.rooms[id]
	delete(f.rooms, id)
	delete(f.secrets, id)
	return ok, nil
}

func (f *fakeCaseAlertRepo) SetSecret(_ context.Context, actor, id string, s repository.CaseAlertSecret, hint string) (bool, error) {
	r, ok := f.rooms[id]
	if !ok {
		return false, nil
	}
	f.actor = actor
	f.secrets[id] = s
	r.WebhookHint = &hint
	r.LastTestAt, r.LastTestStatus = nil, nil
	return true, nil
}

func (f *fakeCaseAlertRepo) Secret(_ context.Context, id string) (*repository.CaseAlertSecret, error) {
	s, ok := f.secrets[id]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

func (f *fakeCaseAlertRepo) Secrets(context.Context) (map[string]repository.CaseAlertSecret, error) {
	return f.secrets, nil
}

func (f *fakeCaseAlertRepo) ClaimTest(_ context.Context, _, id string, at, notBefore time.Time) (bool, error) {
	r, ok := f.rooms[id]
	if !ok || (r.LastTestAt != nil && r.LastTestAt.After(notBefore)) {
		return false, nil
	}
	r.LastTestAt = &at
	return true, nil
}

func (f *fakeCaseAlertRepo) RecordTest(_ context.Context, _, id, status string, at time.Time) error {
	r := f.rooms[id]
	r.LastTestStatus, r.LastTestAt = &status, &at
	return nil
}

func (f *fakeCaseAlertRepo) ListRoutes(context.Context) ([]repository.CaseAlertRouteRow, error) {
	out := []repository.CaseAlertRouteRow{}
	for _, r := range f.routes {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeCaseAlertRepo) SaveRoute(_ context.Context, actor string, w repository.CaseAlertRouteWrite) (repository.CaseAlertRouteRow, error) {
	f.actor, f.route = actor, &w
	return repository.CaseAlertRouteRow{ID: routeSRE, Ladder: w.Ladder, GroupName: w.GroupName, TeamKey: w.TeamKey, RotaCode: w.RotaCode, Enabled: w.Enabled}, nil
}

func (f *fakeCaseAlertRepo) DeleteRoute(_ context.Context, actor, id string) (bool, error) {
	f.actor = actor
	_, ok := f.routes[id]
	delete(f.routes, id)
	return ok, nil
}

func (f *fakeCaseAlertRepo) RosteredTeamFamily(_ context.Context, team string) (string, bool, error) {
	fam, ok := f.teams[team]
	return fam, ok, nil
}

func (f *fakeCaseAlertRepo) ActiveRotaFamily(_ context.Context, code string) (string, bool, error) {
	fam, ok := f.rotas[code]
	return fam, ok, nil
}

// fakeChatPoster records what was posted, and fails when told to.
type fakeChatPoster struct {
	posts   int
	webhook string
	text    string
	fail    error
}

func (p *fakeChatPoster) Post(_ context.Context, webhook, text string) error {
	p.posts++
	p.webhook, p.text = webhook, text
	return p.fail
}

func caseAlertPeople() *stubPagingRepo {
	return &stubPagingRepo{callers: map[string]repository.PagingCaller{
		"sre.rota@wso2.com": {GlobalRoles: []string{"sre_rota_admin"}},
		"cre.rota@wso2.com": {GlobalRoles: []string{"cre_rota_admin"}},
		"sme.rota@wso2.com": {GlobalRoles: []string{"sme_rota_admin"}},
		"all.rota@wso2.com": {GlobalRoles: []string{"cre_rota_admin", "sre_rota_admin", "sme_rota_admin"}},
		"cshead@wso2.com":   {Memberships: []repository.PagingCallerMembership{{TeamKey: "cre-leadership", Role: "cs_head"}}},
		"portal@wso2.com":   {GlobalRoles: []string{"admin"}},
		"crehead@wso2.com":  {Memberships: []repository.PagingCallerMembership{{TeamKey: "cre-leadership", Role: "cre_head"}}},
		"lead@wso2.com":     {Memberships: []repository.PagingCallerMembership{{TeamKey: "apollo", Role: "lead"}}},
		"eng@wso2.com":      {Memberships: []repository.PagingCallerMembership{{TeamKey: "apollo", Role: "engineer"}}},
	}}
}

func newCaseAlerts(repo *fakeCaseAlertRepo, key string) (*caseAlertSettingsService, *fakeChatPoster) {
	svc := NewCaseAlertSettingsService(repo, caseAlertPeople(), alwaysUnrestrictedAccess{}, CaseAlertSettingsConfig{
		AdminRoles: []string{"cs_admin"}, SecretKey: key, SecretKeyID: "1",
		RuntimeClientIDs: []string{"csm-notification-service"}, Environment: "staging",
	}).(*caseAlertSettingsService)
	poster := &fakeChatPoster{}
	svc.poster = poster
	return svc, poster
}

func asPerson(email string, roles ...string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email, Roles: roles})
}

func asService(client string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: client})
}

func wantAPIError[T error](t *testing.T, err error, what string) {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("%s: want %T, got %v", what, target, err)
	}
}

// Who sees the section and which ladders they may change.
func TestCaseAlertSettingsRights(t *testing.T) {
	cases := []struct {
		name    string
		ctx     context.Context
		view    bool
		rooms   []string // ladders whose rooms and values they may see and change
		ladders []string // ladders whose routes they may see and change
		shown   int      // rooms they are sent, of the three (CRE, SRE, Apollo)
	}{
		{"CSM admin by token role", asPerson("someone@wso2.com", "cs_admin"), true, []string{"CRE", "SRE", "SME"}, []string{"CRE", "SRE"}, 3},
		{"portal admin", asPerson("portal@wso2.com"), true, []string{"CRE", "SRE", "SME"}, []string{"CRE", "SRE"}, 3},
		{"all three rota admin roles", asPerson("all.rota@wso2.com"), true, []string{"CRE", "SRE", "SME"}, []string{"CRE", "SRE"}, 3},
		{"SRE rota admin: SRE's only", asPerson("sre.rota@wso2.com"), true, []string{"SRE"}, []string{"SRE"}, 2},
		{"CRE rota admin: CRE's only", asPerson("cre.rota@wso2.com"), true, []string{"CRE"}, []string{"CRE"}, 1},
		{"SME rota admin: SME's only", asPerson("sme.rota@wso2.com"), true, []string{"SME"}, []string{}, 0},
		{"CRE head: CRE's, in full", asPerson("crehead@wso2.com"), true, []string{"CRE"}, []string{"CRE"}, 1},
		{"CS head: CRE's, in full", asPerson("cshead@wso2.com"), true, []string{"CRE"}, []string{"CRE"}, 1},
		{"team lead sees nothing", asPerson("lead@wso2.com"), false, []string{}, []string{}, 0},
		{"engineer sees nothing", asPerson("eng@wso2.com"), false, []string{}, []string{}, 0},
		{"admin role matched exactly", asPerson("eng@wso2.com", "CS_ADMIN"), false, []string{}, []string{}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newFakeCaseAlertRepo()
			svc, _ := newCaseAlerts(repo, testCaseAlertKey)
			got, err := svc.Get(c.ctx)
			if err != nil {
				t.Fatal(err)
			}
			if got.CanManage.View != c.view || !reflect.DeepEqual(got.CanManage.RoomLadders, c.rooms) || !reflect.DeepEqual(got.CanManage.RouteLadders, c.ladders) {
				t.Fatalf("rights = %+v, want view %v rooms %v routes %v", got.CanManage, c.view, c.rooms, c.ladders)
			}
			if !c.view && (repo.listed || len(got.Rooms) != 0 || got.Environment != "") {
				t.Fatalf("somebody who may not see the settings was shown them: %+v", got)
			}
			if c.view && (len(got.Rooms) != c.shown || got.Environment != "staging" || !got.WebhooksEnabled || got.Revision.Saved != 3) {
				t.Fatalf("settings = %+v", got)
			}
			// Another family's rooms are never sent.
			for _, r := range got.Rooms {
				if !slices.Contains(c.rooms, r.Ladder) {
					t.Fatalf("a %s room was sent to %s", r.Ladder, c.name)
				}
			}
		})
	}
}

func TestCaseAlertSettingsRefuseAServiceCredential(t *testing.T) {
	svc, _ := newCaseAlerts(newFakeCaseAlertRepo(), testCaseAlertKey)
	_, err := svc.Get(asService("csm-notification-service"))
	wantAPIError[*apierror.ForbiddenError](t, err, "a service reading the tab")
}

func TestCaseAlertSaveRoom(t *testing.T) {
	yes, chat := true, "chat"
	cases := []struct {
		name string
		ctx  context.Context
		req  domain.SaveCaseAlertRoomRequest
		want string // "", "forbidden", "invalid"
	}{
		{"SRE rota admin, SRE shared room", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "sre", Label: " SRE Case Paging "}, ""},
		{"SRE rota admin, Artemis room, teamless", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "Artemis", Label: "Artemis SRE", Teamless: &yes}, ""},
		{"SRE rota admin, CRE room", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "CRE", Label: "CRE"}, "forbidden"},
		{"CRE head changes CRE", asPerson("crehead@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "CRE", Label: "CRE"}, ""},
		{"CRE head changes nothing of SRE", asPerson("crehead@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", Label: "SRE"}, "forbidden"},
		{"team lead changes nothing", asPerson("lead@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", Label: "SRE"}, "forbidden"},
		{"a CRE team room", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "CRE", TeamKey: "vega", Label: "Vega"}, "invalid"},
		{"a CRE team in an SRE room", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "vega", Label: "Vega"}, "invalid"},
		{"an unknown team", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "nope", Label: "x"}, "invalid"},
		{"teamless shared room", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", Label: "x", Teamless: &yes}, "invalid"},
		{"no label", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", Label: "  "}, "invalid"},
		{"SME rota admin, Asgardeo room", asPerson("sme.rota@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "sme", TeamKey: "Asgardeo", Label: "Asgardeo SME"}, ""},
		{"CSM admin, U2 room", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SME", TeamKey: "u2", Label: "U2 SME"}, ""},
		{"SME rota admin, SRE room", asPerson("sme.rota@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "apollo", Label: "Apollo"}, "forbidden"},
		{"SRE rota admin, SME room", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRoomRequest{Ladder: "SME", TeamKey: "asgardeo", Label: "Asgardeo"}, "forbidden"},
		{"an SME shared room", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SME", Label: "SME"}, "invalid"},
		{"an SRE team in an SME room", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SME", TeamKey: "apollo", Label: "x"}, "invalid"},
		{"an SME team in an SRE room", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "asgardeo", Label: "x"}, "invalid"},
		{"an SME room, teamless", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SME", TeamKey: "asgardeo", Label: "x", Teamless: &yes}, "invalid"},
		{"an SME room with a channel", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "SME", TeamKey: "asgardeo", Label: "x", Channel: &chat}, "invalid"},
		{"a fourth ladder", asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertRoomRequest{Ladder: "PAAS", Label: "x"}, "invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newFakeCaseAlertRepo()
			svc, _ := newCaseAlerts(repo, testCaseAlertKey)
			_, err := svc.SaveRoom(c.ctx, c.req)
			switch c.want {
			case "forbidden":
				wantAPIError[*apierror.ForbiddenError](t, err, c.name)
			case "invalid":
				wantAPIError[*apierror.ValidationError](t, err, c.name)
			default:
				if err != nil {
					t.Fatal(err)
				}
				if repo.saved == nil || repo.actor != auth.IdentityFromContext(c.ctx).UserEmail {
					t.Fatalf("saved %+v by %q", repo.saved, repo.actor)
				}
				if repo.saved.Ladder == "" || strings.TrimSpace(repo.saved.Label) != repo.saved.Label || !repo.saved.Enabled {
					t.Fatalf("saved %+v", repo.saved)
				}
			}
			if c.want != "" && repo.saved != nil {
				t.Fatalf("a refused save was written: %+v", repo.saved)
			}
		})
	}
}

func TestParseChatWebhook(t *testing.T) {
	got, hint, err := parseChatWebhook("  " + testChatWebhook + " ")
	if err != nil {
		t.Fatal(err)
	}
	if got != testChatWebhook || hint != "spaces/AAAAtest01 · …wxyz" {
		t.Fatalf("got %q, hint %q", got, hint)
	}
	bad := map[string]string{
		"http":         strings.Replace(testChatWebhook, "https://", "http://", 1),
		"another host": strings.Replace(testChatWebhook, "chat.googleapis.com", "chat.example.com", 1),
		"look-alike":   strings.Replace(testChatWebhook, "chat.googleapis.com", "chat.googleapis.com.evil.example", 1),
		"user info":    strings.Replace(testChatWebhook, "https://", "https://x@", 1),
		"another path": strings.Replace(testChatWebhook, "/messages", "/members", 1),
		"no token":     strings.Replace(testChatWebhook, "&token=fake-token-wxyz", "", 1),
		"no key":       strings.Replace(testChatWebhook, "key=fake-key&", "", 1),
		"fragment":     testChatWebhook + "#x",
		"empty":        "",
		"not a URL":    "chat room",
		"far too long": testChatWebhook + strings.Repeat("a", maxChatWebhookLen),
	}
	for name, raw := range bad {
		if _, _, err := parseChatWebhook(raw); err == nil {
			t.Errorf("%s: accepted", name)
		} else if strings.Contains(err.Error(), "fake-token") || strings.Contains(err.Error(), "fake-key") {
			t.Errorf("%s: the error quotes the URL: %v", name, err)
		}
	}
}

func TestCaseAlertSealer(t *testing.T) {
	s, err := newCaseAlertSealer(testCaseAlertKey, "1")
	if err != nil || s == nil {
		t.Fatalf("sealer: %v", err)
	}
	sec, err := s.seal(roomApollo, testChatWebhook)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sec.Ciphertext, []byte("fake-token")) || sec.KeyID != "1" {
		t.Fatalf("sealed = %+v", sec)
	}
	if got, err := s.open(roomApollo, sec); err != nil || got != testChatWebhook {
		t.Fatalf("open = %q, %v", got, err)
	}
	if _, err := s.open(roomShared, sec); !errors.Is(err, errSealedElsewhere) {
		t.Fatalf("opened on another room: %v", err)
	}
	other, _ := newCaseAlertSealer(testCaseAlertKey, "2")
	if _, err := other.open(roomApollo, sec); !errors.Is(err, errSealedElsewhere) {
		t.Fatalf("opened under another key id: %v", err)
	}
	wrongKey, _ := newCaseAlertSealer(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)), "1")
	if _, err := wrongKey.open(roomApollo, sec); !errors.Is(err, errSealedElsewhere) {
		t.Fatalf("opened with another key: %v", err)
	}
	if s, err := newCaseAlertSealer("", "1"); s != nil || err != nil {
		t.Fatalf("no key: %v, %v", s, err)
	}
	for _, k := range []string{"not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := newCaseAlertSealer(k, "1"); err == nil {
			t.Errorf("key %q accepted", k)
		}
	}
}

func TestCaseAlertSetWebhook(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	room, err := svc.SetWebhook(asPerson("sre.rota@wso2.com"), roomApollo, domain.SetCaseAlertWebhookRequest{URL: testChatWebhook})
	if err != nil {
		t.Fatal(err)
	}
	if !room.HasWebhook || room.WebhookHint != "spaces/AAAAtest01 · …wxyz" || repo.actor != "sre.rota@wso2.com" {
		t.Fatalf("room = %+v by %q", room, repo.actor)
	}
	// Stored as saved (0237): no key, nothing sealed.
	if got := repo.secrets[roomApollo]; got.URL != testChatWebhook || got.Ciphertext != nil || got.KeyID != "" {
		t.Fatalf("stored %+v", got)
	}

	_, err = svc.SetWebhook(asPerson("cre.rota@wso2.com"), roomApollo, domain.SetCaseAlertWebhookRequest{URL: testChatWebhook})
	wantAPIError[*apierror.ForbiddenError](t, err, "CRE rota admin on an SRE room")
	_, err = svc.SetWebhook(asPerson("sre.rota@wso2.com"), roomApollo, domain.SetCaseAlertWebhookRequest{URL: "https://example.com/hook"})
	wantAPIError[*apierror.ValidationError](t, err, "not a Chat webhook")
	_, err = svc.SetWebhook(asPerson("sre.rota@wso2.com"), "not-a-uuid", domain.SetCaseAlertWebhookRequest{URL: testChatWebhook})
	wantAPIError[*apierror.ValidationError](t, err, "bad room id")
	_, err = svc.SetWebhook(asPerson("sre.rota@wso2.com"), routeSRE, domain.SetCaseAlertWebhookRequest{URL: testChatWebhook})
	wantAPIError[*apierror.NotFoundError](t, err, "no such room")

	// No CASE_ALERTS_SECRET_KEY is needed to save one.
	bare := newFakeCaseAlertRepo()
	noKey, _ := newCaseAlerts(bare, "")
	if _, err = noKey.SetWebhook(asPerson("sre.rota@wso2.com"), roomApollo, domain.SetCaseAlertWebhookRequest{URL: testChatWebhook}); err != nil {
		t.Fatalf("without a key: %v", err)
	}
	if bare.secrets[roomApollo].URL != testChatWebhook {
		t.Fatalf("without a key, stored %+v", bare.secrets[roomApollo])
	}
	if got, _ := noKey.Get(asPerson("sre.rota@wso2.com")); !got.WebhooksEnabled {
		t.Fatal("saving URLs is off without a key")
	}
}

func TestCaseAlertTestCard(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, poster := newCaseAlerts(repo, testCaseAlertKey)
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	ctx := asPerson("sre.rota@wso2.com")

	_, err := svc.SendTestCard(ctx, roomApollo)
	wantAPIError[*apierror.ConflictError](t, err, "no URL saved")

	if _, err := svc.SetWebhook(ctx, roomApollo, domain.SetCaseAlertWebhookRequest{URL: testChatWebhook}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.SendTestCard(ctx, roomApollo)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastTestStatus != "delivered" || poster.posts != 1 || poster.webhook != testChatWebhook || !strings.Contains(poster.text, "Apollo SRE") {
		t.Fatalf("test = %+v, posted %d to the saved URL %v: %q", got, poster.posts, poster.webhook == testChatWebhook, poster.text)
	}

	now = now.Add(90 * time.Second)
	_, err = svc.SendTestCard(ctx, roomApollo)
	wantAPIError[*apierror.TooManyRequestsError](t, err, "within the cooldown")
	if poster.posts != 1 {
		t.Fatal("a second card went out within the cooldown")
	}

	now = now.Add(time.Minute)
	poster.fail = errors.New("the webhook answered 404")
	got, err = svc.SendTestCard(ctx, roomApollo)
	if err != nil || got.LastTestStatus != "failed" || *repo.rooms[roomApollo].LastTestStatus != "failed" {
		t.Fatalf("failed test = %+v, %v", got, err)
	}

	_, err = svc.SendTestCard(asPerson("crehead@wso2.com"), roomApollo)
	wantAPIError[*apierror.ForbiddenError](t, err, "a head may not test")
}

func TestCaseAlertRuntime(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	admin := asPerson("someone@wso2.com", "cs_admin")
	for _, id := range []string{roomApollo, roomCRE} {
		if _, err := svc.SetWebhook(admin, id, domain.SetCaseAlertWebhookRequest{URL: testChatWebhook}); err != nil {
			t.Fatal(err)
		}
	}
	repo.rooms[roomCRE].Enabled = false
	// A URL sealed before 0237, in another environment: it does not open here.
	sealer, err := newCaseAlertSealer(testCaseAlertKey, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if repo.secrets[roomShared], err = sealer.seal(roomShared, testChatWebhook); err != nil {
		t.Fatal(err)
	}
	repo.routes["66666666-6666-6666-6666-666666666666"] = repository.CaseAlertRouteRow{ID: "66666666-6666-6666-6666-666666666666", Ladder: "SRE", GroupName: "off group", TeamKey: "artemis"}

	_, err = svc.Runtime(admin)
	wantAPIError[*apierror.ForbiddenError](t, err, "a person reading the runtime settings")
	_, err = svc.Runtime(asService("csm-integration-service"))
	wantAPIError[*apierror.ForbiddenError](t, err, "a service CASE_ALERTS_RUNTIME_CLIENT_IDS leaves out")

	got, err := svc.Runtime(asService("csm-notification-service"))
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.CaseAlertRuntimeRoom{{Ladder: "SRE", TeamKey: "apollo", Label: "Apollo SRE", Teamless: true, WebhookURL: testChatWebhook}}
	if !reflect.DeepEqual(got.Rooms, want) {
		t.Fatalf("rooms = %+v", got.Rooms)
	}
	if len(got.Routes) != 1 || got.Routes[0].GroupName != "apollo sre group" || got.Revision != 3 {
		t.Fatalf("routes = %+v, revision %d", got.Routes, got.Revision)
	}

	// A URL sealed under this environment's key still opens.
	same, _ := newCaseAlertSealer(testCaseAlertKey, "1")
	if repo.secrets[roomShared], err = same.seal(roomShared, testChatWebhook); err != nil {
		t.Fatal(err)
	}
	if got, err = svc.Runtime(asService("csm-notification-service")); err != nil || len(got.Rooms) != 2 {
		t.Fatalf("with the sealing key: %+v, %v", got.Rooms, err)
	}

	// No setting at all: no key, no CASE_ALERTS_RUNTIME_CLIENT_IDS. Any
	// internal service reads the plain URLs; the sealed one is left out;
	// a person still may not.
	bare := NewCaseAlertSettingsService(repo, caseAlertPeople(), alwaysUnrestrictedAccess{}, CaseAlertSettingsConfig{}).(*caseAlertSettingsService)
	got, err = bare.Runtime(asService("csm-notification-service-dev-client"))
	if err != nil || !reflect.DeepEqual(got.Rooms, want) {
		t.Fatalf("with no settings: %+v, %v", got.Rooms, err)
	}
	_, err = bare.Runtime(admin)
	wantAPIError[*apierror.ForbiddenError](t, err, "a person, with no settings")
}

func TestCaseAlertRecordApplied(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	err := svc.RecordApplied(asService("csm-notification-service"), domain.CaseAlertAppliedReport{Revision: 3, Mode: "Shadow", Summary: map[string]any{"rooms": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.applied == nil || repo.applied.Mode != "shadow" || repo.actor != "service:csm-notification-service" {
		t.Fatalf("applied = %+v by %q", repo.applied, repo.actor)
	}
	err = svc.RecordApplied(asService("csm-notification-service"), domain.CaseAlertAppliedReport{Revision: 3, Mode: "live"})
	wantAPIError[*apierror.ValidationError](t, err, "unknown mode")
	err = svc.RecordApplied(asPerson("someone@wso2.com", "cs_admin"), domain.CaseAlertAppliedReport{Revision: 3, Mode: "db"})
	wantAPIError[*apierror.ForbiddenError](t, err, "a person reporting")
}

func TestCaseAlertSaveRoute(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
		req  domain.SaveCaseAlertRouteRequest
		want string
	}{
		{"SRE team to a team", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: " Artemis SRE Group ", TeamKey: "artemis"}, ""},
		{"SME team to the SaaS rota", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "asgardeo", RotaCode: "sre_saas"}, ""},
		{"CRE alias", asPerson("cre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "CRE", GroupName: "Vega", TeamKey: "vega"}, ""},
		{"SRE: a group no CSM Portal team has", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: " WSO2 IaaS Team ", TeamKey: "apollo"}, ""},
		{"SRE: a group no CSM Portal team has, to a rota", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "ms/pc sre group", RotaCode: "SRE_SAAS"}, ""},
		{"SRE: a CRE leadership team's name", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "CRE Leadership", TeamKey: "apollo"}, "invalid"},
		{"CRE: a group no CSM Portal team has", asPerson("cre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "CRE", GroupName: "WSO2 IaaS Team", TeamKey: "vega"}, "invalid"},
		{"a CRE team on the SRE ladder", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "Vega", TeamKey: "apollo"}, "invalid"},
		{"an SME team on the CRE ladder", asPerson("cre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "CRE", GroupName: "Asgardeo", TeamKey: "vega"}, "invalid"},
		{"SME has no routes here", asPerson("sme.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SME", GroupName: "Asgardeo", TeamKey: "asgardeo"}, "invalid"},
		{"CRE rota admin on SRE", asPerson("cre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "Apollo", TeamKey: "apollo"}, "forbidden"},
		{"team and rota", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "Apollo", TeamKey: "apollo", RotaCode: "SRE_SAAS"}, "invalid"},
		{"neither", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "Apollo"}, "invalid"},
		{"a CRE team on SRE", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "Apollo", TeamKey: "vega"}, "invalid"},
		{"a rota on CRE", asPerson("cre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "CRE", GroupName: "Vega", RotaCode: "CRE_ABT"}, "invalid"},
		{"an unknown rota", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: "Apollo", RotaCode: "NOPE"}, "invalid"},
		{"no group", asPerson("sre.rota@wso2.com"), domain.SaveCaseAlertRouteRequest{Ladder: "SRE", GroupName: " ", TeamKey: "apollo"}, "invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newFakeCaseAlertRepo()
			svc, _ := newCaseAlerts(repo, testCaseAlertKey)
			_, err := svc.SaveRoute(c.ctx, c.req)
			switch c.want {
			case "forbidden":
				wantAPIError[*apierror.ForbiddenError](t, err, c.name)
			case "invalid":
				wantAPIError[*apierror.ValidationError](t, err, c.name)
			default:
				if err != nil {
					t.Fatal(err)
				}
				w := repo.route
				if w == nil || w.GroupName != strings.ToLower(strings.TrimSpace(c.req.GroupName)) || !w.Enabled {
					t.Fatalf("saved %+v", w)
				}
				if c.req.RotaCode != "" && w.RotaCode != "SRE_SAAS" {
					t.Fatalf("rota code not normalised: %+v", w)
				}
			}
			if c.want != "" && repo.route != nil {
				t.Fatalf("a refused route was written: %+v", repo.route)
			}
		})
	}
}

func TestCaseAlertDeleteRoomAndRoute(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	wantAPIError[*apierror.ForbiddenError](t, svc.DeleteRoom(asPerson("cre.rota@wso2.com"), roomApollo), "CRE rota admin removing an SRE room")
	if err := svc.DeleteRoom(asPerson("sre.rota@wso2.com"), roomApollo); err != nil {
		t.Fatal(err)
	}
	wantAPIError[*apierror.NotFoundError](t, svc.DeleteRoom(asPerson("sre.rota@wso2.com"), roomApollo), "removing it twice")

	wantAPIError[*apierror.ForbiddenError](t, svc.DeleteRoute(asPerson("cre.rota@wso2.com"), routeSRE), "CRE rota admin removing an SRE route")
	if err := svc.DeleteRoute(asPerson("sre.rota@wso2.com"), routeSRE); err != nil {
		t.Fatal(err)
	}
	wantAPIError[*apierror.NotFoundError](t, svc.DeleteRoute(asPerson("sre.rota@wso2.com"), routeSRE), "removing it twice")
}
