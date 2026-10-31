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
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These tests pin snCaseService.applyCustomerAttachmentStateTransition through
// the public CreateCaseAttachment path: a customer's upload to a case that is
// Awaiting Info or Solution Proposed moves it to Waiting on WSO2, the same
// rule a customer's comment triggers. The fake backend records every request,
// so what is asserted is whether a state PATCH was actually sent.

const (
	snAttachCustomerEmail = "dave.mendis@example.com"
	snAttachStaffEmail    = "agent.smith@example.com"
)

// snAttachFixture is the fake backend behind each test.
type snAttachFixture struct {
	t              *testing.T
	state          string // case state label
	uploaderEmail  string
	searchFails    bool
	caseSysid      string
	attachmentID   string
	mu             sync.Mutex
	patchBodies    []string
	attachmentPost int
}

func (f *snAttachFixture) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/attachments":
		f.mu.Lock()
		f.attachmentPost++
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message": "Attachment created successfully", "attachment": {"id": "` + f.attachmentID + `", "sizeBytes": 5, "createdOn": "2026-01-02 11:00:00", "createdBy": "someone", "downloadUrl": "https://example.invalid/d"}}`))
	case r.Method == http.MethodPost && r.URL.Path == "/attachments/search":
		if f.searchFails {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message": "boom"}`))
			return
		}
		_, _ = w.Write([]byte(`{"attachments": [{"id": "` + f.attachmentID + `", "referenceId": "` + f.caseSysid + `", "name": "log.txt", "type": "text/plain", "sizeBytes": 5, "createdBy": "` + f.uploaderEmail + `", "createdByFullName": "Uploader", "createdOn": "2026-01-02 11:00:00"}], "offset": 0, "limit": 20, "totalRecords": 1}`))
	case r.Method == http.MethodPatch:
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.patchBodies = append(f.patchBodies, string(body))
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"message": "Case updated successfully", "case": {"id": "` + f.caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe"}}`))
	case strings.HasSuffix(r.URL.Path, "/tags"):
		_, _ = w.Write([]byte(`{"tags":[]}`))
	default:
		_, _ = w.Write([]byte(`{
			"id": "` + f.caseSysid + `",
			"internalId": "WSO2-040", "number": "CS0040001", "title": "t", "description": "d",
			"createdOn": "2026-01-02 10:00:00", "createdBy": "jane.doe@example.com", "createdByFullName": "Jane Doe",
			"project": {"id": "` + sysid32('b') + `", "name": "Project Zeta"},
			"deployment": {"id": "", "name": ""},
			"deployedProduct": {"id": "", "name": "", "version": ""},
			"state": {"id": 1, "label": "` + f.state + `"},
			"watchList": []
		}`))
	}
}

func (f *snAttachFixture) patches() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.patchBodies...)
}

func newSNAttachFixture(t *testing.T, state, uploaderEmail string) *snAttachFixture {
	return &snAttachFixture{t: t, state: state, uploaderEmail: uploaderEmail, caseSysid: sysid32('a'), attachmentID: sysid32('e')}
}

func (f *snAttachFixture) service(customerRoles []string, userSvc SNUserService) CaseService {
	return NewServiceNowCaseService(newTestCaseClient(f.t, f.handler), nil, nil, userSvc, customerRoles, "", nil)
}

func (f *snAttachFixture) request(refType domain.ReferenceType) domain.CreateAttachmentRequest {
	return domain.CreateAttachmentRequest{
		ReferenceID:   sysidToUUID(f.caseSysid),
		ReferenceType: refType,
		Name:          "log.txt",
		Type:          "text/plain",
		File:          "data:text/plain;base64,aGVsbG8=",
	}
}

// failingSNUserService answers every role lookup with an error.
type failingSNUserService struct{ SNUserService }

func (failingSNUserService) SearchUsers(context.Context, domain.SearchUsersRequest) (domain.SearchSNUsersResponse, error) {
	return domain.SearchSNUsersResponse{}, errors.New("lookup failed")
}

func TestSNCaseService_CreateCaseAttachment_CustomerUploadMovesToWaitingOnWSO2(t *testing.T) {
	userSvc := &fakeSNUserService{rolesByEmail: map[string][]string{
		snAttachCustomerEmail: {"customer"},
		snAttachStaffEmail:    {"sn_customerservice_agent"},
	}}
	cases := []struct {
		name          string
		state         string
		uploader      string
		customerRoles []string
		refType       domain.ReferenceType
		wantMove      bool
	}{
		{"customer in Awaiting Info moves", "Awaiting Info", snAttachCustomerEmail, []string{"customer"}, domain.ReferenceTypeCase, true},
		{"customer in Solution Proposed moves", "Solution Proposed", snAttachCustomerEmail, []string{"customer"}, domain.ReferenceTypeCase, true},
		{"customer in Open stays", "Open", snAttachCustomerEmail, []string{"customer"}, domain.ReferenceTypeCase, false},
		{"customer in Work In Progress stays", "Work In Progress", snAttachCustomerEmail, []string{"customer"}, domain.ReferenceTypeCase, false},
		{"customer in Closed stays", "Closed", snAttachCustomerEmail, []string{"customer"}, domain.ReferenceTypeCase, false},
		{"staff in Awaiting Info stays", "Awaiting Info", snAttachStaffEmail, []string{"customer"}, domain.ReferenceTypeCase, false},
		{"empty customerRoles skips", "Awaiting Info", snAttachCustomerEmail, nil, domain.ReferenceTypeCase, false},
		{"deployment attachment never moves a case", "Awaiting Info", snAttachCustomerEmail, []string{"customer"}, domain.ReferenceTypeDeployment, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSNAttachFixture(t, tc.state, tc.uploader)
			svc := f.service(tc.customerRoles, userSvc)

			resp, err := svc.CreateCaseAttachment(contextWithUserIDToken("token"), f.request(tc.refType))
			if err != nil {
				t.Fatalf("CreateCaseAttachment: %v", err)
			}
			if resp.Attachment.ID != sysidToUUID(f.attachmentID) {
				t.Errorf("attachment id = %q, want %q", resp.Attachment.ID, sysidToUUID(f.attachmentID))
			}
			patches := f.patches()
			if !tc.wantMove {
				if len(patches) != 0 {
					t.Fatalf("state PATCH sent %v, want none", patches)
				}
				return
			}
			if len(patches) != 1 || !strings.Contains(strings.ToLower(patches[0]), "state") {
				t.Fatalf("PATCH bodies = %v, want exactly one state update", patches)
			}
		})
	}
}

// TestSNCaseService_CreateCaseAttachment_TransitionFailureNeverFailsTheUpload:
// every way the best-effort rule can break (author unresolvable, role lookup
// failing, the move itself failing) leaves the upload reported as successful.
func TestSNCaseService_CreateCaseAttachment_TransitionFailureNeverFailsTheUpload(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(f *snAttachFixture) SNUserService
	}{
		{"role lookup fails", func(f *snAttachFixture) SNUserService { return failingSNUserService{} }},
		{"author cannot be resolved", func(f *snAttachFixture) SNUserService {
			f.searchFails = true
			return &fakeSNUserService{rolesByEmail: map[string][]string{snAttachCustomerEmail: {"customer"}}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSNAttachFixture(t, "Awaiting Info", snAttachCustomerEmail)
			userSvc := tc.arrange(f)
			svc := f.service([]string{"customer"}, userSvc)

			resp, err := svc.CreateCaseAttachment(contextWithUserIDToken("token"), f.request(domain.ReferenceTypeCase))
			if err != nil {
				t.Fatalf("CreateCaseAttachment returned %v, want the upload to succeed regardless", err)
			}
			if resp.Attachment.ID == "" {
				t.Error("want the created attachment returned")
			}
			if patches := f.patches(); len(patches) != 0 {
				t.Errorf("state PATCH sent %v, want none when the customer cannot be confirmed", patches)
			}
		})
	}
}

// TestSNCaseService_CreateBareCaseAttachment_NoSideEffects: the bare variant
// the dual-write service mirrors through issues only the upload request.
func TestSNCaseService_CreateBareCaseAttachment_NoSideEffects(t *testing.T) {
	f := newSNAttachFixture(t, "Awaiting Info", snAttachCustomerEmail)
	userSvc := &fakeSNUserService{rolesByEmail: map[string][]string{snAttachCustomerEmail: {"customer"}}}
	svc := f.service([]string{"customer"}, userSvc).(*snCaseService)

	if _, err := svc.CreateBareCaseAttachment(contextWithUserIDToken("token"), f.request(domain.ReferenceTypeCase)); err != nil {
		t.Fatalf("CreateBareCaseAttachment: %v", err)
	}
	if patches := f.patches(); len(patches) != 0 {
		t.Fatalf("state PATCH sent %v, want none from the bare variant", patches)
	}
}
