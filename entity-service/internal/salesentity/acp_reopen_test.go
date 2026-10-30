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

package salesentity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

func TestLinkedOpportunitiesByProject_SearchesByProjectAndPages(t *testing.T) {
	var offsets []int
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/linked-opportunities/search", func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			var req map[string]any
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatalf("parse body: %v", err)
			}
			if r.Method != http.MethodPost || req["projectId"] != "a0dE200000CZ9ZNIA1" || req["limit"] != float64(childSearchPageSize) {
				t.Errorf("method=%s body=%s", r.Method, raw)
			}
			if _, ok := req["opportunityId"]; ok {
				t.Errorf("opportunityId sent with a project search: %s", raw)
			}
			offset := int(req["offset"].(float64))
			offsets = append(offsets, offset)
			// A full first page, then a short one: the client must read both.
			n := childSearchPageSize
			if offset > 0 {
				n = 3
			}
			rows := make([]string, n)
			for i := range rows {
				rows[i] = fmt.Sprintf(`{"id":"L%d","projectId":"a0dE200000CZ9ZNIA1","opportunityId":"006%d"}`, offset+i, offset+i)
			}
			_, _ = w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
		})
	})
	got, err := client.LinkedOpportunitiesByProject(context.Background(), "a0dE200000CZ9ZNIA1")
	if err != nil {
		t.Fatalf("LinkedOpportunitiesByProject: %v", err)
	}
	if len(got) != childSearchPageSize+3 {
		t.Fatalf("got %d links, want %d", len(got), childSearchPageSize+3)
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != childSearchPageSize {
		t.Errorf("offsets = %v", offsets)
	}
	if deref(got[0].OpportunityID) != "0060" {
		t.Errorf("first link = %+v", got[0])
	}
}

func TestLinkedOpportunitiesByOpportunity_SearchesByOpportunity(t *testing.T) {
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/linked-opportunities/search", func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			var req map[string]any
			_ = json.Unmarshal(raw, &req)
			if req["opportunityId"] != "006E200000aIb6ZIAS" {
				t.Errorf("body = %s", raw)
			}
			if _, ok := req["projectId"]; ok {
				t.Errorf("projectId sent with an opportunity search: %s", raw)
			}
			_, _ = w.Write([]byte(`[{"id":"L1","projectId":"P1","opportunityId":"006E200000aIb6ZIAS"},{"id":"L2","projectId":"P2","opportunityId":"006E200000aIb6ZIAS"}]`))
		})
	})
	got, err := client.LinkedOpportunitiesByOpportunity(context.Background(), "006E200000aIb6ZIAS")
	if err != nil {
		t.Fatalf("LinkedOpportunitiesByOpportunity: %v", err)
	}
	if len(got) != 2 || deref(got[1].ProjectID) != "P2" {
		t.Errorf("links = %+v", got)
	}
}

func TestInvoicesByOpportunity_EmptyIsNoInvoices(t *testing.T) {
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/invoices/search", func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"opportunityId":"006E200000aIb6ZIAS"`) {
				t.Errorf("body = %s", raw)
			}
			_, _ = w.Write([]byte(`[]`))
		})
	})
	got, err := client.InvoicesByOpportunity(context.Background(), "006E200000aIb6ZIAS")
	if err != nil {
		t.Fatalf("InvoicesByOpportunity: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("invoices = %+v, want none", got)
	}
}

func TestChildSearch_StopsAfterTheLastPage(t *testing.T) {
	calls := 0
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/invoices/search", func(w http.ResponseWriter, _ *http.Request) {
			calls++
			rows := make([]string, childSearchPageSize)
			for i := range rows {
				rows[i] = `{"id":"I"}`
			}
			_, _ = w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
		})
	})
	if _, err := client.InvoicesByOpportunity(context.Background(), "006"); err == nil {
		t.Fatal("a search that never ends was followed without an error")
	}
	if calls != childSearchMaxPages {
		t.Errorf("calls = %d, want %d", calls, childSearchMaxPages)
	}
}

func TestUpdateProjectStatus_PatchesTheProject(t *testing.T) {
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/projects/a0dE200000CZ9ZNIA1", func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPatch || string(raw) != `{"status":"Open"}` {
				t.Errorf("method=%s body=%s", r.Method, raw)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"Project updated successfully"}`))
		})
	})
	if err := client.UpdateProjectStatus(context.Background(), "a0dE200000CZ9ZNIA1", ProjectStatusOpen); err != nil {
		t.Fatalf("UpdateProjectStatus: %v", err)
	}
}

func TestUpdateProjectStatus_Errors(t *testing.T) {
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/projects/missing", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		mux.HandleFunc("/projects/down", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})
	})
	var nf *apierror.NotFoundError
	if err := client.UpdateProjectStatus(context.Background(), "missing", ProjectStatusOpen); !errors.As(err, &nf) {
		t.Errorf("404: err = %v, want NotFoundError", err)
	}
	var su *apierror.ServiceUnavailableError
	if err := client.UpdateProjectStatus(context.Background(), "down", ProjectStatusOpen); !errors.As(err, &su) {
		t.Errorf("502: err = %v, want ServiceUnavailableError", err)
	}
	if err := client.UpdateProjectStatus(context.Background(), "  ", ProjectStatusOpen); err == nil {
		t.Error("a blank project id was sent")
	}
}

func TestCustomer_DecodesThePrimaryPartnerAndOwnerName(t *testing.T) {
	var c Customer
	raw := `{"id":"001","owner":{"name":"Alex Owner","email":"owner@example.com"},"primaryPartnerAccountId":"001P"}`
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	if deref(c.PrimaryPartnerAccountID) != "001P" || c.Owner == nil || deref(c.Owner.Name) != "Alex Owner" {
		t.Errorf("customer = %+v owner = %+v", c, c.Owner)
	}
}
