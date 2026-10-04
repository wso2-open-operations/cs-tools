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

package handler

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
)

// slowMembershipService takes `delay` to answer every write, standing in for
// a membership write held up by slow Salesforce calls.
type slowMembershipService struct{ delay time.Duration }

func (s slowMembershipService) Invite(context.Context, string, domain.CreateProjectMembershipRequest) (domain.ProjectMembership, error) {
	time.Sleep(s.delay)
	return domain.ProjectMembership{ProjectContactID: "pc-1", State: domain.MembershipStateInvited}, nil
}

func (s slowMembershipService) ValidateInvitation(context.Context, string, domain.ValidateProjectMembershipRequest) (domain.ProjectMembershipValidation, error) {
	time.Sleep(s.delay)
	return domain.ProjectMembershipValidation{Valid: true}, nil
}

func (s slowMembershipService) UpdateRoles(context.Context, string, string, domain.UpdateProjectMembershipRolesRequest) (domain.ProjectMembership, error) {
	time.Sleep(s.delay)
	return domain.ProjectMembership{}, nil
}

func (s slowMembershipService) Deactivate(context.Context, string, string) error {
	time.Sleep(s.delay)
	return nil
}

func (s slowMembershipService) ResendInvitation(context.Context, string, string) error {
	time.Sleep(s.delay)
	return nil
}

// serveWithWriteTimeout runs handler behind the production logging and
// recovery wrappers on a real server whose WriteTimeout is writeTimeout, and
// POSTs to it.
func serveWithWriteTimeout(t *testing.T, writeTimeout time.Duration, register func(*http.ServeMux)) (*http.Response, error) {
	t.Helper()
	mux := http.NewServeMux()
	register(mux)
	srv := &http.Server{Handler: middleware.Recovery(middleware.Logger(mux)), WriteTimeout: writeTimeout}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	client := &http.Client{Timeout: 5 * time.Second}
	return client.Post("http://"+ln.Addr().String()+"/projects/p-1/contacts", "application/json",
		bytes.NewBufferString(`{"email":"jane@acme.com","roles":["Portal user"]}`))
}

// TestInviteProjectContact_AnswersPastTheServerWriteTimeout: an invitation
// that outlasts the server-wide WriteTimeout must still get its answer to the
// caller, not a closed connection after the write has committed.
func TestInviteProjectContact_AnswersPastTheServerWriteTimeout(t *testing.T) {
	h := NewProjectMembershipHandler(slowMembershipService{delay: 300 * time.Millisecond})

	resp, err := serveWithWriteTimeout(t, 100*time.Millisecond, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /projects/{id}/contacts", h.InviteProjectContact)
	})

	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", resp.StatusCode, body)
	}
}

// TestServerWriteTimeout_DropsASlowHandlerWithoutTheExtension is the control:
// without extendDeadlines the same slow handler loses its response, which
// is the failure the extension exists to prevent.
func TestServerWriteTimeout_DropsASlowHandlerWithoutTheExtension(t *testing.T) {
	resp, err := serveWithWriteTimeout(t, 100*time.Millisecond, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /projects/{id}/contacts", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusCreated)
		})
	})

	if err == nil {
		resp.Body.Close()
		t.Fatalf("got status %d, want the connection dropped", resp.StatusCode)
	}
}

// refusingMembershipService answers every dry run with a refusal, or fails
// it with err when set.
type refusingMembershipService struct {
	slowMembershipService
	err error
}

func (s refusingMembershipService) ValidateInvitation(context.Context, string, domain.ValidateProjectMembershipRequest) (domain.ProjectMembershipValidation, error) {
	if s.err != nil {
		return domain.ProjectMembershipValidation{}, s.err
	}
	return domain.ProjectMembershipValidation{Reason: domain.MembershipValidationForbidden, Message: "domain not allowed"}, nil
}

// TestValidateProjectContact_RefusalIsA200: a refused invitation is the
// answer to the question asked, so it comes back 200 with valid=false; only
// a failed check is an error status.
func TestValidateProjectContact_RefusalIsA200(t *testing.T) {
	cases := []struct {
		name       string
		svc        refusingMembershipService
		wantStatus int
		wantBody   string
	}{
		{"refused", refusingMembershipService{}, http.StatusOK, `"valid":false`},
		{"check failed", refusingMembershipService{err: &apierror.ServiceUnavailableError{Msg: "salesentity down"}}, http.StatusServiceUnavailable, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewProjectMembershipHandler(tc.svc)
			mux := http.NewServeMux()
			mux.HandleFunc("POST /projects/{id}/contacts/validate", h.ValidateProjectContact)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/projects/p-1/contacts/validate", bytes.NewBufferString(`{"email":"jane@acme.com"}`))
			req.Header.Set("Content-Type", "application/json")
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body = %s, want it to contain %s", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

// ctxAwareMembershipService answers Invite after `delay`, or with the
// context's error if the context it was given ends first.
type ctxAwareMembershipService struct {
	slowMembershipService
	delay time.Duration
}

func (s ctxAwareMembershipService) Invite(ctx context.Context, _ string, _ domain.CreateProjectMembershipRequest) (domain.ProjectMembership, error) {
	select {
	case <-time.After(s.delay):
		return domain.ProjectMembership{ProjectContactID: "pc-1", State: domain.MembershipStateInvited}, nil
	case <-ctx.Done():
		return domain.ProjectMembership{}, ctx.Err()
	}
}

// TestInviteProjectContact_OutlivesTheRequestTimeout: the server-wide request
// timeout cancels the request context well before a slow membership write is
// done. The handler must run the write on its own, longer deadline so the
// work is not cancelled half-way.
func TestInviteProjectContact_OutlivesTheRequestTimeout(t *testing.T) {
	h := NewProjectMembershipHandler(ctxAwareMembershipService{delay: 150 * time.Millisecond})

	req := httptest.NewRequest(http.MethodPost, "/projects/p-1/contacts",
		bytes.NewBufferString(`{"email":"jane.doe@example.com","roles":["Portal user"]}`))
	ctx, cancel := context.WithTimeout(req.Context(), 20*time.Millisecond)
	defer cancel()
	rec := httptest.NewRecorder()

	h.InviteProjectContact(rec, req.WithContext(ctx))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 — the write was cut short by the request timeout; body %s", rec.Code, rec.Body.String())
	}
}
