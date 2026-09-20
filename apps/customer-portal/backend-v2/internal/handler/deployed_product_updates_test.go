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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// The Manage Products dialog's Update History tab saves by PATCHing `updates`
// on its own, with no other field set. That request used to be rejected before
// it reached entity-service, because the handler's "detail fields" check did
// not count updates — so saving update history was impossible through this
// backend while it worked through the v1 Ballerina one.

const (
	updDeploymentID = "44444444-4444-4444-4444-444444444444"
	updProductID    = "55555555-5555-5555-5555-555555555555"
)

type updFakeEntity struct {
	entityDeployedProductClient
	gotUpdate entity.UpdateDeployedProductRequest
	calls     int
}

func (f *updFakeEntity) UpdateDeployedProduct(_ context.Context, _ string, req entity.UpdateDeployedProductRequest) (entity.UpdateDeployedProductResponse, error) {
	f.calls++
	f.gotUpdate = req
	return entity.UpdateDeployedProductResponse{}, nil
}

func patchDeployedProductMux(fake *updFakeEntity) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /deployments/{deploymentId}/products/{id}",
		NewDeployedProductHandler(fake).PatchDeployedProduct)
	return mux
}

func patchDeployedProduct(fake *updFakeEntity, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	patchDeployedProductMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/deployments/"+updDeploymentID+"/products/"+updProductID, body))
	return w
}

// TestPatchDeployedProduct_AcceptsUpdatesOnly is the regression this file
// exists for.
func TestPatchDeployedProduct_AcceptsUpdatesOnly(t *testing.T) {
	fake := &updFakeEntity{}
	w := patchDeployedProduct(fake, `{"updates":[{"updateLevel":59,"date":"2026-09-16"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if fake.calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", fake.calls)
	}
	if fake.gotUpdate.Updates == nil {
		t.Fatal("updates were not forwarded upstream")
	}
	got := *fake.gotUpdate.Updates
	if len(got) != 1 || got[0].UpdateLevel != 59 || got[0].Date != "2026-09-16" {
		t.Fatalf("forwarded updates = %+v, want one entry level 59 on 2026-09-16", got)
	}
}

// TestPatchDeployedProduct_ForwardsEmptyUpdatesArray covers clearing the
// history. An empty array must reach entity-service as `[]`, not vanish —
// encoding/json's omitempty drops a zero-length slice, which is why the field
// is a pointer to a slice.
func TestPatchDeployedProduct_ForwardsEmptyUpdatesArray(t *testing.T) {
	fake := &updFakeEntity{}
	w := patchDeployedProduct(fake, `{"updates":[]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if fake.gotUpdate.Updates == nil {
		t.Fatal("an explicit empty updates array was treated as absent")
	}
	if len(*fake.gotUpdate.Updates) != 0 {
		t.Fatalf("forwarded %d entries, want 0", len(*fake.gotUpdate.Updates))
	}

	// And it must survive serialisation as "[]" rather than being omitted.
	encoded, err := json.Marshal(fake.gotUpdate)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(raw["updates"]) != "[]" {
		t.Fatalf("updates serialised as %q, want []", string(raw["updates"]))
	}
}

// TestPatchDeployedProduct_OmitsAbsentUpdates is the other half: a request that
// does not mention updates must not send the field, or every cores/tps edit
// would wipe the product's update history.
func TestPatchDeployedProduct_OmitsAbsentUpdates(t *testing.T) {
	fake := &updFakeEntity{}
	w := patchDeployedProduct(fake, `{"cores":4}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if fake.gotUpdate.Updates != nil {
		t.Fatal("updates must stay nil when the request did not mention them")
	}
	encoded, err := json.Marshal(fake.gotUpdate)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := raw["updates"]; present {
		t.Fatalf("updates must be omitted entirely: %s", encoded)
	}
}

// TestPatchDeployedProduct_RejectsUpdatesWithActive keeps the handler in step
// with entity-service, which refuses detail fields alongside a deactivation.
func TestPatchDeployedProduct_RejectsUpdatesWithActive(t *testing.T) {
	fake := &updFakeEntity{}
	w := patchDeployedProduct(fake, `{"updates":[{"updateLevel":59,"date":"2026-09-16"}],"active":false}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if fake.calls != 0 {
		t.Fatal("upstream was called for an invalid combination")
	}
}

// TestPatchDeployedProduct_RejectsEmptyBody — neither branch satisfied is still
// an error, unchanged by adding updates to the check.
func TestPatchDeployedProduct_RejectsEmptyBody(t *testing.T) {
	fake := &updFakeEntity{}
	w := patchDeployedProduct(fake, `{}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if fake.calls != 0 {
		t.Fatal("upstream was called for an empty request")
	}
}
