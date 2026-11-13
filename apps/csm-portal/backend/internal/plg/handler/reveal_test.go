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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/cloudclient"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/service"
)

const revealOrg = "11111111-1111-1111-1111-111111111111"

type storedIdentities struct{}

func (storedIdentities) Identities(_ context.Context, ids []string) ([]domain.OrganizationIdentity, error) {
	name, email := "acme", "owner@acme.com"
	out := []domain.OrganizationIdentity{}
	for _, id := range ids {
		if id == revealOrg {
			out = append(out, domain.OrganizationIdentity{OrganizationID: id, OrganizationName: &name, RegisteredEmail: &email})
		}
	}
	return out, nil
}

type noCloud struct{}

func (noCloud) GetOrg(context.Context, string) (*cloudclient.Org, error) { return nil, nil }
func (noCloud) GetUserEmail(context.Context, string) (*cloudclient.UserEmail, error) {
	return nil, nil
}

type refuseAll struct{}

func (refuseAll) Allow(string) (bool, time.Duration, bool) {
	return false, 1500 * time.Millisecond, false
}

func reveal(h *Handlers, id string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /plg/organizations/{organizationId}/reveal", h.RevealOrganization)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/plg/organizations/"+id+"/reveal", nil))
	return rec
}

func revealHandlers() *Handlers {
	return (&Handlers{}).WithCompanyResolver(service.NewCompanyResolver(storedIdentities{}, noCloud{}))
}

func TestReveal_OneOrganisationIsNeverCached(t *testing.T) {
	rec := reveal(revealHandlers(), revealOrg)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	var got domain.CompanyIdentity
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.OrganizationID != revealOrg {
		t.Errorf("body = %s (%v)", rec.Body, err)
	}
}

func TestReveal_UnknownOrBadIDs(t *testing.T) {
	if rec := reveal(revealHandlers(), "22222222-2222-2222-2222-222222222222"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown organisation: status = %d", rec.Code)
	}
	if rec := reveal(revealHandlers(), "not-a-uuid"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id: status = %d", rec.Code)
	}
}

func TestReveal_OverTheLimitIsRefusedWithRetryAfter(t *testing.T) {
	rec := reveal(revealHandlers().WithRevealLimit(refuseAll{}), revealOrg)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "2" {
		t.Errorf("status = %d, Retry-After = %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a refusal must not be cacheable either")
	}
}
