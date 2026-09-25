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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/usermanagement"
)

const (
	testProjectUUID = "11111111-2222-3333-4444-555555555555"
	testProjectSfID = "a0Pxx0000000001"
	testInvitee     = "shayan+e2e1@wso2.com"
	testCaller      = "admin@acme.com"
)

// fakeProjectResolver resolves the project's Salesforce Id, which only the
// pre-cutover path needs.
type fakeProjectResolver struct{ calls int }

func (f *fakeProjectResolver) GetProject(context.Context, string) (entity.ProjectDetailsView, error) {
	f.calls++
	return entity.ProjectDetailsView{ID: testProjectUUID, SfID: testProjectSfID}, nil
}

// fakeLegacyContacts is the pre-cutover onboarding service. It records every
// call, reads included, so a test can assert the flag kept traffic away from
// it.
type fakeLegacyContacts struct {
	calls     []string
	projectID string
}

func (f *fakeLegacyContacts) GetProjectContacts(_ context.Context, projectID string) ([]usermanagement.Contact, error) {
	f.calls = append(f.calls, "list")
	f.projectID = projectID
	return []usermanagement.Contact{{ID: "003xx", Email: testCaller, LastName: "Admin", IsCsAdmin: true}}, nil
}

func (f *fakeLegacyContacts) CreateProjectContact(_ context.Context, projectID string, _ usermanagement.OnBoardContactPayload) (usermanagement.Membership, error) {
	f.calls = append(f.calls, "create")
	f.projectID = projectID
	return usermanagement.Membership{}, nil
}

func (f *fakeLegacyContacts) RemoveProjectContact(_ context.Context, projectID, _, _ string) (usermanagement.Membership, error) {
	f.calls = append(f.calls, "remove")
	f.projectID = projectID
	return usermanagement.Membership{}, nil
}

func (f *fakeLegacyContacts) UpdateMembershipRole(_ context.Context, projectID, _ string, _ usermanagement.MembershipRolePayload) (usermanagement.Membership, error) {
	f.calls = append(f.calls, "update")
	f.projectID = projectID
	return usermanagement.Membership{}, nil
}

func (f *fakeLegacyContacts) ValidateProjectContact(context.Context, usermanagement.ValidationPayload) (*usermanagement.Contact, bool, error) {
	return nil, false, nil
}

// fakeMemberships is entity-service's side. err, when set, is returned from
// every write so the upstream-error mapping can be exercised. contacts is the
// project's database contact list, which both the list endpoint and the
// admin check read; listing is not recorded in calls, so calls holds writes
// only.
type fakeMemberships struct {
	calls       []string
	projectID   string
	email       string
	create      entity.CreateProjectMembershipRequest
	update      entity.UpdateProjectMembershipRolesRequest
	result      entity.ProjectMembership
	err         error
	contacts    []entity.ProjectContact
	contactsErr error
	listed      int
	// seen records, for each write, what the context looked like when the
	// call arrived: whether it was already cancelled, whether it carried a
	// deadline, and whether the request's values survived.
	seen []writeContext
}

type writeContext struct {
	err         error
	hasDeadline bool
	hasUser     bool
}

func (f *fakeMemberships) record(ctx context.Context) {
	_, hasDeadline := ctx.Deadline()
	f.seen = append(f.seen, writeContext{err: ctx.Err(), hasDeadline: hasDeadline, hasUser: middleware.UserInfoFromContext(ctx) != nil})
}

func (f *fakeMemberships) ListProjectContacts(context.Context, string) ([]entity.ProjectContact, error) {
	f.listed++
	return f.contacts, f.contactsErr
}

func (f *fakeMemberships) CreateProjectMembership(ctx context.Context, projectID string, req entity.CreateProjectMembershipRequest) (entity.ProjectMembership, error) {
	f.record(ctx)
	f.calls = append(f.calls, "create")
	f.projectID, f.create = projectID, req
	return f.result, f.err
}

func (f *fakeMemberships) UpdateProjectMembershipRoles(ctx context.Context, projectID, email string, req entity.UpdateProjectMembershipRolesRequest) (entity.ProjectMembership, error) {
	f.record(ctx)
	f.calls = append(f.calls, "update")
	f.projectID, f.email, f.update = projectID, email, req
	return f.result, f.err
}

func (f *fakeMemberships) DeactivateProjectMembership(ctx context.Context, projectID, email string) error {
	f.record(ctx)
	f.calls = append(f.calls, "deactivate")
	f.projectID, f.email = projectID, email
	return f.err
}

func (f *fakeMemberships) ResendProjectMembershipInvitation(ctx context.Context, projectID, email string) error {
	f.record(ctx)
	f.calls = append(f.calls, "resend")
	f.projectID, f.email = projectID, email
	return f.err
}

type contactFakes struct {
	resolver    *fakeProjectResolver
	legacy      *fakeLegacyContacts
	memberships *fakeMemberships
}

// newContactMux wires the handler into the same routes main.go registers, so
// the tests exercise real path-value decoding (the "+" in the invitee).
// memberships is passed as a nil interface when withClient is false, which
// is the pre-cutover wiring.
// The caller is an active CS admin of the project by default, so each test
// that is not about authorization gets past the check.
func newContactMux(portalContacts, withClient bool) (*http.ServeMux, contactFakes) {
	f := contactFakes{
		resolver: &fakeProjectResolver{},
		legacy:   &fakeLegacyContacts{},
		memberships: &fakeMemberships{contacts: []entity.ProjectContact{
			{Email: testCaller, RegistrationState: "REGISTERED", Roles: []string{"ADMIN", "PORTAL_USER"}},
		}},
	}
	var mc membershipsClient
	if withClient {
		mc = f.memberships
	}
	h := NewContactHandler(f.resolver, f.legacy, mc, portalContacts)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /projects/{id}/contacts", h.GetProjectContacts)
	mux.HandleFunc("POST /projects/{id}/contacts", h.CreateProjectContact)
	mux.HandleFunc("DELETE /projects/{id}/contacts/{email}", h.RemoveProjectContact)
	mux.HandleFunc("PATCH /projects/{id}/contacts/{email}", h.UpdateProjectContactRole)
	mux.HandleFunc("POST /projects/{id}/contacts/{email}/resend-invitation", h.ResendProjectContactInvitation)
	return mux, f
}

func serveContact(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	return serveContactWithContext(context.Background(), mux, method, path, body)
}

// serveContactWithContext is serveContact on a caller-supplied base context,
// so a test can hand the handler a request that is already cancelled.
func serveContactWithContext(ctx context.Context, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{UserID: "u-1", Email: testCaller}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

const inviteBody = `{"contactEmail":"shayan+e2e1@wso2.com","contactFirstName":"E2E","contactLastName":"One","isPortalUser":true,"isSecurityContact":true}`

func contactsPath() string { return "/projects/" + testProjectUUID + "/contacts" }

// escapedInviteePath is how the webapp sends the address: path-escaped, so
// the handler must see the decoded "shayan+e2e1@wso2.com".
func escapedInviteePath() string { return contactsPath() + "/shayan%2Be2e1@wso2.com" }

func TestCreateProjectContact_PortalContactsOnUsesEntityService(t *testing.T) {
	mux, f := newContactMux(true, true)
	f.memberships.result = entity.ProjectMembership{
		ProjectContactID: "pc-1", ContactSfID: "003xx", Email: testInvitee,
		State: "INVITED", Roles: []string{"Portal user", "Security Contact"},
	}

	rec := serveContact(mux, http.MethodPost, contactsPath(), inviteBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if !reflect.DeepEqual(f.memberships.calls, []string{"create"}) {
		t.Fatalf("entity calls = %v, want [create]", f.memberships.calls)
	}
	if f.memberships.projectID != testProjectUUID {
		t.Errorf("projectID = %q, want the project UUID %q, not its Salesforce Id", f.memberships.projectID, testProjectUUID)
	}
	want := entity.CreateProjectMembershipRequest{
		Email: testInvitee, FirstName: "E2E", LastName: "One",
		Roles: []string{"Portal user", "Security Contact"},
	}
	if !reflect.DeepEqual(f.memberships.create, want) {
		t.Errorf("create body = %+v, want %+v", f.memberships.create, want)
	}
	if len(f.legacy.calls) != 0 || f.resolver.calls != 0 {
		t.Errorf("legacy calls = %v, GetProject calls = %d; want none", f.legacy.calls, f.resolver.calls)
	}

	var got dto.Membership
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "pc-1" || got.State != "INVITED" || !got.IsPortalUser || !got.IsSecurityContact || got.IsLead || got.IsCsAdmin {
		t.Errorf("response = %+v", got)
	}
}

func TestCreateProjectContact_PortalContactsOffUsesLegacyService(t *testing.T) {
	mux, f := newContactMux(false, true)

	rec := serveContact(mux, http.MethodPost, contactsPath(), inviteBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if len(f.memberships.calls) != 0 {
		t.Errorf("entity calls = %v with the flag off, want none", f.memberships.calls)
	}
	if !reflect.DeepEqual(f.legacy.calls, []string{"create"}) || f.legacy.projectID != testProjectSfID {
		t.Errorf("legacy calls = %v on %q, want [create] on %q", f.legacy.calls, f.legacy.projectID, testProjectSfID)
	}
}

// TestNewContactHandler_FlagWithoutClientStaysOnLegacy covers the
// misconfiguration guard: the flag on with no entity client behind it must
// not select a path that would nil-dereference.
func TestNewContactHandler_FlagWithoutClientStaysOnLegacy(t *testing.T) {
	mux, f := newContactMux(true, false)

	rec := serveContact(mux, http.MethodPost, contactsPath(), inviteBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if !reflect.DeepEqual(f.legacy.calls, []string{"create"}) {
		t.Errorf("legacy calls = %v, want [create]", f.legacy.calls)
	}
}

func TestCreateProjectContact_PortalContactsOnPassesUpstreamError(t *testing.T) {
	mux, f := newContactMux(true, true)
	f.memberships.err = apierror.NewUpstreamError(http.StatusConflict, []byte(`{"message":"Contact is already a member of this project."}`))

	rec := serveContact(mux, http.MethodPost, contactsPath(), inviteBody)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "already a member") {
		t.Errorf("body = %s, want entity-service's message", rec.Body)
	}
}

func TestUpdateProjectContactRole_PortalContactsOnUsesEntityService(t *testing.T) {
	mux, f := newContactMux(true, true)
	f.memberships.result = entity.ProjectMembership{ProjectContactID: "pc-1", State: "INVITED", Roles: []string{"Lead"}}

	rec := serveContact(mux, http.MethodPatch, escapedInviteePath(), `{"isLead":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if !reflect.DeepEqual(f.memberships.calls, []string{"update"}) {
		t.Fatalf("entity calls = %v, want [update]", f.memberships.calls)
	}
	if f.memberships.projectID != testProjectUUID || f.memberships.email != testInvitee {
		t.Errorf("target = %q/%q, want %q/%q", f.memberships.projectID, f.memberships.email, testProjectUUID, testInvitee)
	}
	if !reflect.DeepEqual(f.memberships.update.Roles, []string{"Lead"}) {
		t.Errorf("roles = %q, want [Lead]", f.memberships.update.Roles)
	}
	if len(f.legacy.calls) != 0 || f.resolver.calls != 0 {
		t.Errorf("legacy calls = %v, GetProject calls = %d; want none", f.legacy.calls, f.resolver.calls)
	}
}

func TestUpdateProjectContactRole_PortalContactsOffUsesLegacyService(t *testing.T) {
	mux, f := newContactMux(false, true)

	rec := serveContact(mux, http.MethodPatch, escapedInviteePath(), `{"isLead":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if len(f.memberships.calls) != 0 || !reflect.DeepEqual(f.legacy.calls, []string{"update"}) {
		t.Errorf("entity calls = %v, legacy calls = %v; want none and [update]", f.memberships.calls, f.legacy.calls)
	}
}

func TestRemoveProjectContact_PortalContactsOnDeactivates(t *testing.T) {
	mux, f := newContactMux(true, true)

	rec := serveContact(mux, http.MethodDelete, escapedInviteePath(), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if !reflect.DeepEqual(f.memberships.calls, []string{"deactivate"}) {
		t.Fatalf("entity calls = %v, want [deactivate]", f.memberships.calls)
	}
	if f.memberships.projectID != testProjectUUID || f.memberships.email != testInvitee {
		t.Errorf("target = %q/%q, want %q/%q", f.memberships.projectID, f.memberships.email, testProjectUUID, testInvitee)
	}
	if len(f.legacy.calls) != 0 || f.resolver.calls != 0 {
		t.Errorf("legacy calls = %v, GetProject calls = %d; want none", f.legacy.calls, f.resolver.calls)
	}
}

func TestRemoveProjectContact_PortalContactsOffUsesLegacyService(t *testing.T) {
	mux, f := newContactMux(false, true)

	rec := serveContact(mux, http.MethodDelete, escapedInviteePath(), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if len(f.memberships.calls) != 0 || !reflect.DeepEqual(f.legacy.calls, []string{"remove"}) {
		t.Errorf("entity calls = %v, legacy calls = %v; want none and [remove]", f.memberships.calls, f.legacy.calls)
	}
}

// TestResendProjectContactInvitation_FlagOff404s: resend has no pre-cutover
// equivalent, so with the flag off (or no client behind it) it must answer
// 404 and reach neither service.
func TestResendProjectContactInvitation_FlagOff404s(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		portalContacts, withClient bool
	}{
		{"flag off", false, true},
		{"flag on without client", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux, f := newContactMux(tc.portalContacts, tc.withClient)

			rec := serveContact(mux, http.MethodPost, escapedInviteePath()+"/resend-invitation", "")

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if len(f.memberships.calls) != 0 || len(f.legacy.calls) != 0 {
				t.Errorf("entity calls = %v, legacy calls = %v; want none", f.memberships.calls, f.legacy.calls)
			}
		})
	}
}

func TestResendProjectContactInvitation_PortalContactsOn(t *testing.T) {
	mux, f := newContactMux(true, true)

	rec := serveContact(mux, http.MethodPost, escapedInviteePath()+"/resend-invitation", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if !reflect.DeepEqual(f.memberships.calls, []string{"resend"}) {
		t.Fatalf("entity calls = %v, want [resend]", f.memberships.calls)
	}
	if f.memberships.projectID != testProjectUUID || f.memberships.email != testInvitee {
		t.Errorf("target = %q/%q, want %q/%q", f.memberships.projectID, f.memberships.email, testProjectUUID, testInvitee)
	}
}

func TestResendProjectContactInvitation_InvalidProjectID(t *testing.T) {
	mux, f := newContactMux(true, true)

	rec := serveContact(mux, http.MethodPost, "/projects/not-a-uuid/contacts/"+testInvitee+"/resend-invitation", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(f.memberships.calls) != 0 {
		t.Errorf("entity calls = %v, want none", f.memberships.calls)
	}
}

func TestResendProjectContactInvitation_PassesUpstreamNotFound(t *testing.T) {
	mux, f := newContactMux(true, true)
	f.memberships.err = apierror.NewUpstreamError(http.StatusNotFound, []byte(`{"message":"Membership not found."}`))

	rec := serveContact(mux, http.MethodPost, escapedInviteePath()+"/resend-invitation", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestResendProjectContactInvitation_Unauthenticated(t *testing.T) {
	mux, f := newContactMux(true, true)
	req := httptest.NewRequest(http.MethodPost, escapedInviteePath()+"/resend-invitation", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if len(f.memberships.calls) != 0 {
		t.Errorf("entity calls = %v, want none", f.memberships.calls)
	}
}

// portalWriteRequests is one request per write on the entity-service path,
// for the authorization tests that apply to all four alike.
var portalWriteRequests = []struct {
	name, method, path, body string
}{
	{"invite", http.MethodPost, contactsPath(), inviteBody},
	{"role change", http.MethodPatch, escapedInviteePath(), `{"isLead":true}`},
	{"remove", http.MethodDelete, escapedInviteePath(), ""},
	{"resend", http.MethodPost, escapedInviteePath() + "/resend-invitation", ""},
}

// TestPortalContacts_RefuseCallerWhoIsNotProjectAdmin is the check entity-service
// leaves to the portal: a signed-in user who is not an active CS admin of the
// project must get 403 and nothing may reach entity-service.
func TestPortalContacts_RefuseCallerWhoIsNotProjectAdmin(t *testing.T) {
	callers := []struct {
		name     string
		contacts []entity.ProjectContact
	}{
		{"not on the project", []entity.ProjectContact{{Email: "someone@acme.com", RegistrationState: "REGISTERED", Roles: []string{"ADMIN"}}}},
		{"on the project without admin", []entity.ProjectContact{{Email: testCaller, RegistrationState: "REGISTERED", Roles: []string{"PORTAL_USER", "LEAD_USER"}}}},
		{"deactivated admin", []entity.ProjectContact{{Email: testCaller, RegistrationState: "Deactivated", Roles: []string{"ADMIN"}}}},
		{"empty contact list", nil},
	}
	for _, caller := range callers {
		for _, rq := range portalWriteRequests {
			t.Run(caller.name+"/"+rq.name, func(t *testing.T) {
				mux, f := newContactMux(true, true)
				f.memberships.contacts = caller.contacts

				rec := serveContact(mux, rq.method, rq.path, rq.body)

				if rec.Code != http.StatusForbidden {
					t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
				}
				if len(f.memberships.calls) != 0 || len(f.legacy.calls) != 0 {
					t.Errorf("entity writes = %v, legacy calls = %v; want none", f.memberships.calls, f.legacy.calls)
				}
			})
		}
	}
}

// TestPortalContacts_AdminEmailMatchIgnoresCase: Salesforce does not keep the
// casing the address was typed in, and a case mismatch must not lock an
// admin out of their own project.
func TestPortalContacts_AdminEmailMatchIgnoresCase(t *testing.T) {
	for _, rq := range portalWriteRequests {
		t.Run(rq.name, func(t *testing.T) {
			mux, f := newContactMux(true, true)
			f.memberships.contacts = []entity.ProjectContact{{Email: " Admin@ACME.com ", RegistrationState: "REGISTERED", Roles: []string{"admin"}}}

			rec := serveContact(mux, rq.method, rq.path, rq.body)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
			}
			if len(f.memberships.calls) != 1 {
				t.Errorf("entity calls = %v, want exactly one", f.memberships.calls)
			}
		})
	}
}

// TestPortalContacts_ContactLookupFailureBlocksWrite: when the admin check
// cannot be answered, the write must not go ahead.
func TestPortalContacts_ContactLookupFailureBlocksWrite(t *testing.T) {
	for _, rq := range portalWriteRequests {
		t.Run(rq.name, func(t *testing.T) {
			mux, f := newContactMux(true, true)
			f.memberships.contactsErr = apierror.NewUpstreamError(http.StatusBadGateway, nil)

			rec := serveContact(mux, rq.method, rq.path, rq.body)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503; body %s", rec.Code, rec.Body)
			}
			if len(f.memberships.calls) != 0 {
				t.Errorf("entity calls = %v, want none", f.memberships.calls)
			}
		})
	}
}

// TestGetProjectContacts_PortalContactsOnReadsDatabase: with the flag on the
// list comes from entity-service's database search, never from the
// pre-cutover service, and is rendered in the portal's own Contact shape.
func TestGetProjectContacts_PortalContactsOnReadsDatabase(t *testing.T) {
	mux, f := newContactMux(true, true)
	userID, name := "u-9", "Jane Q Doe"
	f.memberships.contacts = []entity.ProjectContact{
		{ID: &userID, Name: &name, Email: "jane@acme.com", RegistrationState: "INVITED", Roles: []string{"PORTAL_USER", "SECURITY_CONTACT", "BUSINESS_CONTACT"}},
		{Email: "noname@acme.com", RegistrationState: "REGISTERED", Roles: []string{"ADMIN", "LEAD_USER"}},
	}

	rec := serveContact(mux, http.MethodGet, contactsPath(), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if len(f.legacy.calls) != 0 || f.resolver.calls != 0 {
		t.Errorf("legacy calls = %v, GetProject calls = %d; want none", f.legacy.calls, f.resolver.calls)
	}
	var got []dto.Contact
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d contacts, want 2", len(got))
	}
	jane := got[0]
	if jane.ID != "u-9" || jane.FirstName == nil || *jane.FirstName != "Jane" || jane.LastName != "Q Doe" {
		t.Errorf("jane id/name = %q/%v/%q, want u-9/Jane/Q Doe", jane.ID, jane.FirstName, jane.LastName)
	}
	if !jane.IsPortalUser || !jane.IsSecurityContact || jane.IsLead || jane.IsCsAdmin {
		t.Errorf("jane flags = %+v", jane)
	}
	if jane.MembershipStatus == nil || *jane.MembershipStatus != "INVITED" {
		t.Errorf("jane status = %v, want INVITED", jane.MembershipStatus)
	}
	noname := got[1]
	if noname.ID != "noname@acme.com" || noname.FirstName != nil || noname.LastName != "" {
		t.Errorf("unlinked row id/name = %q/%v/%q, want the email as id and no name", noname.ID, noname.FirstName, noname.LastName)
	}
	if !noname.IsCsAdmin || !noname.IsLead || noname.IsPortalUser {
		t.Errorf("unlinked row flags = %+v", noname)
	}
}

func TestGetProjectContacts_PortalContactsOffUsesLegacyService(t *testing.T) {
	mux, f := newContactMux(false, true)

	rec := serveContact(mux, http.MethodGet, contactsPath(), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if f.memberships.listed != 0 {
		t.Errorf("database list read %d times with the flag off, want 0", f.memberships.listed)
	}
	if !reflect.DeepEqual(f.legacy.calls, []string{"list"}) || f.legacy.projectID != testProjectSfID {
		t.Errorf("legacy calls = %v on %q, want [list] on %q", f.legacy.calls, f.legacy.projectID, testProjectSfID)
	}
}

func TestGetProjectContacts_PortalContactsOnPassesUpstreamError(t *testing.T) {
	mux, f := newContactMux(true, true)
	f.memberships.contactsErr = apierror.NewUpstreamError(http.StatusServiceUnavailable, nil)

	rec := serveContact(mux, http.MethodGet, contactsPath(), "")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// TestPortalContacts_WritesSurviveRequestCancellation: a write updates
// Salesforce and then commits Postgres, so a closed tab or dropped connection
// must not abandon it halfway. Each write reaches entity-service on a context
// that is not cancelled, carries its own deadline, and still has the
// request's values (the user, and with it the forwarded token and
// correlation id).
func TestPortalContacts_WritesSurviveRequestCancellation(t *testing.T) {
	for _, rq := range portalWriteRequests {
		t.Run(rq.name, func(t *testing.T) {
			mux, f := newContactMux(true, true)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			serveContactWithContext(ctx, mux, rq.method, rq.path, rq.body)

			if len(f.memberships.seen) != 1 {
				t.Fatalf("entity writes = %d, want 1", len(f.memberships.seen))
			}
			got := f.memberships.seen[0]
			if got.err != nil {
				t.Errorf("write context already done: %v", got.err)
			}
			if !got.hasDeadline {
				t.Error("write context has no deadline")
			}
			if !got.hasUser {
				t.Error("write context lost the request's values")
			}
		})
	}
}

// TestCreateProjectContact_PortalTimeoutAnswers202: when the portal stops
// waiting, entity-service keeps going and commits, so the invite is reported
// as still processing rather than failed.
func TestCreateProjectContact_PortalTimeoutAnswers202(t *testing.T) {
	mux, f := newContactMux(true, true)
	f.memberships.err = fmt.Errorf("entity: POST /projects/x/contacts: %w", context.DeadlineExceeded)

	rec := serveContact(mux, http.MethodPost, contactsPath(), inviteBody)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body %s", rec.Code, rec.Body)
	}
	var got invitationProcessingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != invitationStatusProcessing || got.Message == "" {
		t.Errorf("body = %+v, want status %s with a message", got, invitationStatusProcessing)
	}
}

// TestUpdateProjectContactRole_PortalTimeoutStaysAnError: only the invite has
// a pending row the webapp can resolve by refreshing; other writes keep
// reporting a timeout as a failure.
func TestUpdateProjectContactRole_PortalTimeoutStaysAnError(t *testing.T) {
	mux, f := newContactMux(true, true)
	f.memberships.err = fmt.Errorf("entity: PATCH: %w", context.DeadlineExceeded)

	rec := serveContact(mux, http.MethodPatch, escapedInviteePath(), `{"isLead":true}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
