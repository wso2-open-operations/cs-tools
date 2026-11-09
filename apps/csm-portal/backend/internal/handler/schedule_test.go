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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// mockEntityScheduleClient stands in for entity-service, recording what the
// handler forwarded so a test can assert the body reached it untouched.
type mockEntityScheduleClient struct {
	catalogueFn   func(ctx context.Context) ([]byte, error)
	assignmentsFn func(ctx context.Context, body []byte) ([]byte, error)
	allocationsFn func(ctx context.Context, body []byte) ([]byte, error)
	dismissFn     func(ctx context.Context, body []byte) ([]byte, error)
	absencesFn    func(ctx context.Context, body []byte) ([]byte, error)
	onDutyFn      func(ctx context.Context, at string) ([]byte, error)

	createFn   func(ctx context.Context, body []byte) ([]byte, error)
	updateFn   func(ctx context.Context, id string, body []byte) ([]byte, error)
	deleteFn   func(ctx context.Context, id, note string) ([]byte, error)
	activityFn func(ctx context.Context, teamKey, from, to string) ([]byte, error)
	leadFn     func(ctx context.Context) ([]byte, error)

	deleteAbsenceFn func(ctx context.Context, id, note string) ([]byte, error)
	createKindFn    func(ctx context.Context, body []byte) ([]byte, error)
	deleteKindFn    func(ctx context.Context, code string) ([]byte, error)
	applyFn         func(ctx context.Context, body []byte) ([]byte, error)
	absenceFn       func(ctx context.Context, body []byte) ([]byte, error)

	pagingChainFn  func(ctx context.Context, family string) ([]byte, error)
	pagingMemberFn func(ctx context.Context, id string, body []byte) ([]byte, error)

	putContactFn    func(ctx context.Context, userID string, body []byte) ([]byte, error)
	deleteContactFn func(ctx context.Context, userID string) ([]byte, error)
	testContactFn   func(ctx context.Context, userID string) ([]byte, error)
}

func (m *mockEntityScheduleClient) PutPagingContact(ctx context.Context, userID string, body []byte) ([]byte, error) {
	if m.putContactFn != nil {
		return m.putContactFn(ctx, userID, body)
	}
	return []byte(`{"masked":"+94•••••123"}`), nil
}

func (m *mockEntityScheduleClient) DeletePagingContact(ctx context.Context, userID string) ([]byte, error) {
	if m.deleteContactFn != nil {
		return m.deleteContactFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockEntityScheduleClient) TestPagingContact(ctx context.Context, userID string) ([]byte, error) {
	if m.testContactFn != nil {
		return m.testContactFn(ctx, userID)
	}
	return []byte(`{"lastTestStatus":"pending"}`), nil
}

func (m *mockEntityScheduleClient) GetPagingChain(ctx context.Context, family string) ([]byte, error) {
	if m.pagingChainFn != nil {
		return m.pagingChainFn(ctx, family)
	}
	return []byte(`{"family":"` + family + `","members":[],"count":0}`), nil
}

func (m *mockEntityScheduleClient) UpdatePagingMember(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.pagingMemberFn != nil {
		return m.pagingMemberFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityScheduleClient) GetScheduleEditMarkers(context.Context, string, string) ([]byte, error) {
	return nil, nil
}

func (m *mockEntityScheduleClient) DeleteScheduleAbsence(ctx context.Context, id, note string) ([]byte, error) {
	if m.deleteAbsenceFn == nil {
		return nil, nil
	}
	return m.deleteAbsenceFn(ctx, id, note)
}

func (m *mockEntityScheduleClient) DeleteScheduleAbsenceKind(ctx context.Context, code string) ([]byte, error) {
	if m.deleteKindFn == nil {
		return nil, nil
	}
	return m.deleteKindFn(ctx, code)
}

func (m *mockEntityScheduleClient) CreateScheduleAbsenceKind(ctx context.Context, body []byte) ([]byte, error) {
	if m.createKindFn == nil {
		return nil, nil
	}
	return m.createKindFn(ctx, body)
}

func (m *mockEntityScheduleClient) ApplyScheduleAbsence(ctx context.Context, body []byte) ([]byte, error) {
	if m.absenceFn == nil {
		return nil, nil
	}
	return m.absenceFn(ctx, body)
}

func (m *mockEntityScheduleClient) ApplyScheduleRange(ctx context.Context, body []byte) ([]byte, error) {
	if m.applyFn == nil {
		return nil, nil
	}
	return m.applyFn(ctx, body)
}

func (m *mockEntityScheduleClient) GetMyLeadTeams(ctx context.Context) ([]byte, error) {
	if m.leadFn == nil {
		return nil, nil
	}
	return m.leadFn(ctx)
}

func (m *mockEntityScheduleClient) CreateScheduleAssignment(ctx context.Context, body []byte) ([]byte, error) {
	if m.createFn == nil {
		return nil, nil
	}
	return m.createFn(ctx, body)
}

func (m *mockEntityScheduleClient) UpdateScheduleAssignment(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.updateFn == nil {
		return nil, nil
	}
	return m.updateFn(ctx, id, body)
}

func (m *mockEntityScheduleClient) DeleteScheduleAssignment(ctx context.Context, id, note string) ([]byte, error) {
	if m.deleteFn == nil {
		return nil, nil
	}
	return m.deleteFn(ctx, id, note)
}

func (m *mockEntityScheduleClient) GetScheduleActivity(ctx context.Context, teamKey, from, to string) ([]byte, error) {
	if m.activityFn == nil {
		return nil, nil
	}
	return m.activityFn(ctx, teamKey, from, to)
}

func (m *mockEntityScheduleClient) GetScheduleCatalogue(ctx context.Context) ([]byte, error) {
	if m.catalogueFn != nil {
		return m.catalogueFn(ctx)
	}
	return []byte(`{"zones":[],"shifts":[],"absenceKinds":[]}`), nil
}

func (m *mockEntityScheduleClient) SearchScheduleAssignments(ctx context.Context, body []byte) ([]byte, error) {
	if m.assignmentsFn != nil {
		return m.assignmentsFn(ctx, body)
	}
	return []byte(`{"assignments":[],"count":0}`), nil
}

func (m *mockEntityScheduleClient) SearchScheduleAbsences(ctx context.Context, body []byte) ([]byte, error) {
	if m.absencesFn != nil {
		return m.absencesFn(ctx, body)
	}
	return []byte(`{"absences":[],"count":0}`), nil
}

func (m *mockEntityScheduleClient) DismissScheduleEngagementAllocation(ctx context.Context, body []byte) ([]byte, error) {
	if m.dismissFn != nil {
		return m.dismissFn(ctx, body)
	}
	return []byte(`{"days":0}`), nil
}

func (m *mockEntityScheduleClient) SearchScheduleEngagementAllocations(ctx context.Context, body []byte) ([]byte, error) {
	if m.allocationsFn != nil {
		return m.allocationsFn(ctx, body)
	}
	return []byte(`{"allocations":[],"count":0}`), nil
}

func (m *mockEntityScheduleClient) GetScheduleOnDuty(ctx context.Context, at string) ([]byte, error) {
	if m.onDutyFn != nil {
		return m.onDutyFn(ctx, at)
	}
	return []byte(`{"assignments":[],"count":0}`), nil
}

func TestSearchScheduleAssignments(t *testing.T) {
	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects a body over the size limit", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search",
			strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})

	t.Run("rejects a body that is not JSON", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search",
			strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("forwards the search to entity-service unchanged", func(t *testing.T) {
		const payload = `{"from":"2026-09-21","to":"2026-09-27","family":"SRE","includeOvernight":true}`
		var captured []byte
		client := &mockEntityScheduleClient{
			assignmentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = body
				return []byte(`{"assignments":[{"id":"a"}],"count":1}`), nil
			},
		}
		h := NewScheduleHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(captured) != payload {
			t.Fatalf("body was altered in transit:\n got %s\nwant %s", captured, payload)
		}
		if !strings.Contains(w.Body.String(), `"count":1`) {
			t.Fatalf("upstream response did not reach the caller: %s", w.Body.String())
		}
	})

	t.Run("surfaces an upstream failure rather than pretending it worked", func(t *testing.T) {
		client := &mockEntityScheduleClient{
			assignmentsFn: func(context.Context, []byte) ([]byte, error) {
				return nil, errors.New("upstream is down")
			},
		}
		h := NewScheduleHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		if w.Code == http.StatusOK {
			t.Fatal("an upstream error must not come back as 200")
		}
	})
}

func TestGetScheduleCatalogue(t *testing.T) {
	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.GetScheduleCatalogue(w, httptest.NewRequest(http.MethodGet, "/team-schedule/catalogue", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("returns the catalogue", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			catalogueFn: func(context.Context) ([]byte, error) {
				return []byte(`{"zones":[{"code":"TZ1"}],"shifts":[],"absenceKinds":[]}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleCatalogue(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/catalogue", nil)))
		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), "TZ1") {
			t.Fatalf("catalogue did not reach the caller: %s", w.Body.String())
		}
	})
}

func TestGetScheduleOnDuty(t *testing.T) {
	t.Run("passes the instant through when one is asked for", func(t *testing.T) {
		var gotAt string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			onDutyFn: func(_ context.Context, at string) ([]byte, error) {
				gotAt = at
				return []byte(`{"assignments":[],"count":0}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleOnDuty(w, withUser(httptest.NewRequest(http.MethodGet,
			"/team-schedule/on-duty?at=2026-09-21T22%3A15%3A00Z", nil)))
		assertStatus(t, w, http.StatusOK)
		if gotAt != "2026-09-21T22:15:00Z" {
			t.Fatalf("want the instant forwarded verbatim, got %q", gotAt)
		}
	})

	t.Run("asks about now when no instant is given", func(t *testing.T) {
		var gotAt = "unset"
		h := NewScheduleHandler(&mockEntityScheduleClient{
			onDutyFn: func(_ context.Context, at string) ([]byte, error) {
				gotAt = at
				return []byte(`{"assignments":[],"count":0}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleOnDuty(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/on-duty", nil)))
		assertStatus(t, w, http.StatusOK)
		if gotAt != "" {
			t.Fatalf("want an empty instant so the service defaults to now, got %q", gotAt)
		}
	})
}

func TestSearchScheduleEngagementAllocations(t *testing.T) {
	const path = "/team-schedule/engagement-allocations/search"
	const payload = `{"userIds":["00000000-0000-0000-0000-000000000000"],"from":"2026-10-19","to":"2026-10-25"}`

	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.SearchScheduleEngagementAllocations(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload)))
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects a body that is not JSON", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.SearchScheduleEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(`not-json`))))
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects a body over the size limit", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.SearchScheduleEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path,
			strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))))
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})

	t.Run("forwards the search unchanged and returns the response as sent", func(t *testing.T) {
		const upstream = `{"allocations":[{"id":"00000000-0000-0000-0000-000000000001","userId":"00000000-0000-0000-0000-000000000000","engagementId":"00000000-0000-0000-0000-000000000002","startDate":"2026-10-19","endDate":"2026-10-21","accountName":"Example Corp","engagementType":"CONSULTANCY","modeOfDelivery":"ONSITE","utilization":50}],"count":1}`
		var captured []byte
		h := NewScheduleHandler(&mockEntityScheduleClient{
			allocationsFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = body
				return []byte(upstream), nil
			},
		})
		w := httptest.NewRecorder()
		h.SearchScheduleEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))))

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(captured) != payload {
			t.Fatalf("body was altered in transit:\n got %s\nwant %s", captured, payload)
		}
		if w.Body.String() != upstream {
			t.Fatalf("response was altered (enums must stay raw):\n got %s\nwant %s", w.Body.String(), upstream)
		}
	})

	t.Run("an empty result is an empty list, not null", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.SearchScheduleEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))))
		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), `"allocations":[]`) || strings.Contains(w.Body.String(), "null") {
			t.Fatalf("want an empty list, got %s", w.Body.String())
		}
	})

	t.Run("maps upstream failures", func(t *testing.T) {
		for _, c := range []struct {
			name       string
			err        error
			wantStatus int
		}{
			{"400 from the window or id limits", &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"window too large"}`}, http.StatusBadRequest},
			{"403 stays forbidden", &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"message":"no"}`}, http.StatusForbidden},
			{"500 stays generic", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"message":"pq: relation does not exist"}`}, http.StatusInternalServerError},
			{"transport error is not a 200", errors.New("upstream is down"), 0},
		} {
			t.Run(c.name, func(t *testing.T) {
				h := NewScheduleHandler(&mockEntityScheduleClient{
					allocationsFn: func(context.Context, []byte) ([]byte, error) { return nil, c.err },
				})
				w := httptest.NewRecorder()
				h.SearchScheduleEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))))
				if w.Code == http.StatusOK {
					t.Fatal("an upstream error must not come back as 200")
				}
				if c.wantStatus != 0 {
					assertStatus(t, w, c.wantStatus)
				}
				if strings.Contains(w.Body.String(), "pq:") {
					t.Fatalf("internal detail leaked: %s", w.Body.String())
				}
			})
		}
	})
}

func TestSearchScheduleAbsences(t *testing.T) {
	t.Run("forwards the search and returns the response", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			absencesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"absences":[{"id":"x"}],"count":1}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.SearchScheduleAbsences(w, withUser(httptest.NewRequest(http.MethodPost,
			"/team-schedule/absences/search", strings.NewReader(`{"from":"2026-09-21","to":"2026-09-21"}`))))
		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), `"count":1`) {
			t.Fatalf("upstream response did not reach the caller: %s", w.Body.String())
		}
	})
}

// A refused edit keeps entity-service's reason -- the lead needs to know the
// engineer is not on that team, or already holds an overlapping window -- and
// anything else stays generic.
func TestScheduleWriteKeepsTheRefusalReason(t *testing.T) {
	for _, c := range []struct {
		name       string
		upstream   *apierror.Error
		wantStatus int
		wantMsg    string
	}{
		{"409 overlap", &apierror.Error{StatusCode: http.StatusConflict, Body: `{"code":409,"message":"this person already has a window that overlaps this one on 2026-10-10"}`},
			http.StatusConflict, "this person already has a window that overlaps this one on 2026-10-10"},
		{"403 not on the team", &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"code":403,"message":"that engineer is not on americas, so their rota is not yours to change"}`},
			http.StatusForbidden, "that engineer is not on americas, so their rota is not yours to change"},
		{"500 stays generic", &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"message":"pq: relation does not exist"}`},
			http.StatusInternalServerError, "Failed to change the rota."},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := NewScheduleHandler(&mockEntityScheduleClient{
				applyFn: func(context.Context, []byte) ([]byte, error) { return nil, c.upstream },
			})
			r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/apply",
				strings.NewReader(`{"userId":"u","teamKey":"vega","from":"2026-10-10","to":"2026-10-10"}`)))
			w := httptest.NewRecorder()
			h.ApplyScheduleRange(w, r)
			if w.Code != c.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, c.wantStatus)
			}
			if !strings.Contains(w.Body.String(), c.wantMsg) {
				t.Fatalf("body = %s, want it to carry %q", w.Body.String(), c.wantMsg)
			}
		})
	}
}

func TestGetPagingChain(t *testing.T) {
	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.GetPagingChain(w, httptest.NewRequest(http.MethodGet, "/team-schedule/paging-chain?family=CRE", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("passes the family through", func(t *testing.T) {
		var got string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			pagingChainFn: func(_ context.Context, family string) ([]byte, error) {
				got = family
				return []byte(`{"family":"SRE","members":[],"count":0,"canEdit":{"heads":false}}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetPagingChain(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/paging-chain?family=SRE", nil)))
		assertStatus(t, w, http.StatusOK)
		if got != "SRE" {
			t.Fatalf("family forwarded = %q, want SRE", got)
		}
		if !strings.Contains(w.Body.String(), `"canEdit"`) {
			t.Fatalf("response not passed through: %s", w.Body.String())
		}
	})

	t.Run("refuses an unknown family", func(t *testing.T) {
		called := false
		h := NewScheduleHandler(&mockEntityScheduleClient{
			pagingChainFn: func(context.Context, string) ([]byte, error) { called = true; return nil, nil },
		})
		w := httptest.NewRecorder()
		h.GetPagingChain(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/paging-chain?family=x%27", nil)))
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("an unknown family reached entity-service")
		}
	})

	t.Run("upstream failure is generic", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			pagingChainFn: func(context.Context, string) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"message":"pq: boom"}`}
			},
		})
		w := httptest.NewRecorder()
		h.GetPagingChain(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/paging-chain", nil)))
		assertStatus(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "pq") {
			t.Fatalf("upstream detail leaked: %s", w.Body.String())
		}
	})
}

func TestUpdatePagingMember(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	patch := func(h *ScheduleHandler, pathID, body string, authed bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPatch, "/team-schedule/paging-chain/members/"+pathID, strings.NewReader(body))
		r.SetPathValue("id", pathID)
		if authed {
			r = withUser(r)
		}
		w := httptest.NewRecorder()
		h.UpdatePagingMember(w, r)
		return w
	}

	t.Run("requires an authenticated user", func(t *testing.T) {
		assertStatus(t, patch(NewScheduleHandler(&mockEntityScheduleClient{}), id, `{"responderRank":1}`, false), http.StatusUnauthorized)
	})

	t.Run("refuses an id that is not a UUID", func(t *testing.T) {
		assertStatus(t, patch(NewScheduleHandler(&mockEntityScheduleClient{}), "not-a-uuid", `{"responderRank":1}`, true), http.StatusBadRequest)
	})

	t.Run("refuses a body that is not JSON", func(t *testing.T) {
		assertStatus(t, patch(NewScheduleHandler(&mockEntityScheduleClient{}), id, `{`, true), http.StatusBadRequest)
	})

	t.Run("forwards the id and body unchanged", func(t *testing.T) {
		var gotID, gotBody string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			pagingMemberFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				gotID, gotBody = id, string(body)
				return []byte(`{"membershipId":"` + id + `","responderRank":2}`), nil
			},
		})
		w := patch(h, id, `{"responderRank":2}`, true)
		assertStatus(t, w, http.StatusOK)
		if gotID != id || gotBody != `{"responderRank":2}` {
			t.Fatalf("forwarded id=%q body=%q", gotID, gotBody)
		}
	})

	t.Run("keeps a refusal reason", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			pagingMemberFn: func(context.Context, string, []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"code":403,"message":"only this team's lead may set its responders"}`}
			},
		})
		w := patch(h, id, `{"responderRank":1}`, true)
		assertStatus(t, w, http.StatusForbidden)
		if !strings.Contains(w.Body.String(), "only this team's lead may set its responders") {
			t.Fatalf("refusal reason lost: %s", w.Body.String())
		}
	})
}

// The paging chain says, per member, whether their profile has a mobile
// number -- leaving every other field untouched -- and a member whose lookup
// failed simply has no field.
func TestGetPagingChain_AddsProfilePhone(t *testing.T) {
	upstream := `{"family":"CRE","count":3,"canEdit":{"heads":true},"members":[
	  {"membershipId":"m1","email":"Ann@Example.com","responderRank":1,"pagingPhone":null},
	  {"membershipId":"m2","email":"bob@example.com","responderRank":0,"pagingPhone":{"masked":"+94•••••123"}},
	  {"membershipId":"m3","email":"cat@example.com","responderRank":2}]}`
	sc := &fakeSCIM{phones: map[string]string{"ann@example.com": "+1", "bob@example.com": ""}, fail: map[string]bool{"cat@example.com": true}}
	h := NewScheduleHandler(&mockEntityScheduleClient{
		pagingChainFn: func(context.Context, string) ([]byte, error) { return []byte(upstream), nil },
	}).WithPagingPhones(NewPagingPhoneChecker(sc))
	w := httptest.NewRecorder()
	h.GetPagingChain(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/paging-chain?family=CRE", nil)))
	assertStatus(t, w, http.StatusOK)

	got := decodeJSON[struct {
		Count   int              `json:"count"`
		CanEdit map[string]any   `json:"canEdit"`
		Members []map[string]any `json:"members"`
	}](t, w)
	if got.Count != 3 || got.CanEdit["heads"] != true || len(got.Members) != 3 {
		t.Fatalf("the rest of the response changed: %s", w.Body.String())
	}
	if got.Members[0]["hasProfilePhone"] != true || got.Members[1]["hasProfilePhone"] != false {
		t.Fatalf("hasProfilePhone = %v, %v; want true, false", got.Members[0]["hasProfilePhone"], got.Members[1]["hasProfilePhone"])
	}
	if _, ok := got.Members[2]["hasProfilePhone"]; ok {
		t.Fatal("a failed lookup must leave the field out")
	}
	if got.Members[1]["pagingPhone"].(map[string]any)["masked"] != "+94•••••123" || got.Members[0]["responderRank"] != float64(1) {
		t.Fatalf("member fields changed: %s", w.Body.String())
	}
}

// entity-service's own hasProfilePhone (from "user".phone) stands where it
// says true; where it says false SCIM is asked only while the fallback is on.
func TestGetPagingChain_ProfilePhoneFromEntityService(t *testing.T) {
	upstream := `{"members":[
	  {"membershipId":"m1","email":"db@example.com","hasProfilePhone":true},
	  {"membershipId":"m2","email":"asg@example.com","hasProfilePhone":false}]}`
	read := func(phones *PagingPhoneChecker) []map[string]any {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			pagingChainFn: func(context.Context, string) ([]byte, error) { return []byte(upstream), nil },
		}).WithPagingPhones(phones)
		w := httptest.NewRecorder()
		h.GetPagingChain(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/paging-chain?family=CRE", nil)))
		assertStatus(t, w, http.StatusOK)
		return decodeJSON[struct {
			Members []map[string]any `json:"members"`
		}](t, w).Members
	}

	sc := &fakeSCIM{phones: map[string]string{"asg@example.com": "+94770000002"}}
	got := read(NewPagingPhoneChecker(sc))
	if got[0]["hasProfilePhone"] != true || got[1]["hasProfilePhone"] != true {
		t.Fatalf("fallback on: hasProfilePhone = %v, %v; want true, true", got[0]["hasProfilePhone"], got[1]["hasProfilePhone"])
	}
	if strings.Join(sc.asked, ",") != "asg@example.com" {
		t.Errorf("SCIM asked about %v; want only the member entity-service has no number for", sc.asked)
	}

	off := &fakeSCIM{phones: map[string]string{"asg@example.com": "+94770000002"}}
	got = read(NewPagingPhoneChecker(off).WithSCIMFallback(false))
	if got[0]["hasProfilePhone"] != true || got[1]["hasProfilePhone"] != false || len(off.asked) != 0 {
		t.Fatalf("fallback off: hasProfilePhone = %v, %v and SCIM asked %v; want true, false, nobody",
			got[0]["hasProfilePhone"], got[1]["hasProfilePhone"], off.asked)
	}
}

func TestGetPagingChain_UnreadableResponsePassesThrough(t *testing.T) {
	h := NewScheduleHandler(&mockEntityScheduleClient{
		pagingChainFn: func(context.Context, string) ([]byte, error) { return []byte(`{"members":"odd"}`), nil },
	}).WithPagingPhones(NewPagingPhoneChecker(&fakeSCIM{}))
	w := httptest.NewRecorder()
	h.GetPagingChain(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/paging-chain", nil)))
	assertStatus(t, w, http.StatusOK)
	if w.Body.String() != `{"members":"odd"}` {
		t.Fatalf("body = %s, want it unchanged", w.Body.String())
	}
}

func TestPagingContactRoutes(t *testing.T) {
	const userID = "22222222-2222-2222-2222-222222222222"
	call := func(h *ScheduleHandler, method, path, id, body string, authed bool, fn func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
		var rd *strings.Reader
		if body != "" {
			rd = strings.NewReader(body)
		} else {
			rd = strings.NewReader("")
		}
		r := httptest.NewRequest(method, path, rd)
		r.SetPathValue("userId", id)
		if authed {
			r = withUser(r)
		}
		w := httptest.NewRecorder()
		fn(w, r)
		return w
	}
	put := func(h *ScheduleHandler, id, body string, authed bool) *httptest.ResponseRecorder {
		return call(h, http.MethodPut, "/team-schedule/paging-contacts/"+id, id, body, authed, h.PutPagingContact)
	}

	t.Run("PUT requires a user, a UUID and an E.164 number", func(t *testing.T) {
		called := false
		h := NewScheduleHandler(&mockEntityScheduleClient{
			putContactFn: func(context.Context, string, []byte) ([]byte, error) { called = true; return nil, nil },
		})
		assertStatus(t, put(h, userID, `{"phone":"+94771234123"}`, false), http.StatusUnauthorized)
		assertStatus(t, put(h, "not-a-uuid", `{"phone":"+94771234123"}`, true), http.StatusBadRequest)
		for _, bad := range []string{`{}`, `{"phone":"0771234123"}`, `{"phone":"+0771234123"}`, `{"phone":"+12345"}`, `{"phone":"+1234567890123456"}`, `{"phone":12}`, `{`} {
			assertStatus(t, put(h, userID, bad, true), http.StatusBadRequest)
		}
		if called {
			t.Fatal("a refused request reached entity-service")
		}
	})

	t.Run("PUT forwards only the number and returns the paging phone", func(t *testing.T) {
		var gotID, gotBody string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			putContactFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				gotID, gotBody = id, string(body)
				return []byte(`{"masked":"+94•••••123","phone":"+94771234123","setBy":"lead@example.com"}`), nil
			},
		})
		w := put(h, userID, `{"phone":"+94771234123","extra":true}`, true)
		assertStatus(t, w, http.StatusOK)
		if gotID != userID || gotBody != `{"phone":"+94771234123"}` {
			t.Fatalf("forwarded id=%q body=%q", gotID, gotBody)
		}
		if !strings.Contains(w.Body.String(), "+94•••••123") {
			t.Fatalf("response not passed through: %s", w.Body.String())
		}
	})

	t.Run("PUT keeps a 403 reason", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			putContactFn: func(context.Context, string, []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"message":"only this person's team lead may set their paging number"}`}
			},
		})
		w := put(h, userID, `{"phone":"+94771234123"}`, true)
		assertStatus(t, w, http.StatusForbidden)
		if !strings.Contains(w.Body.String(), "only this person's team lead") {
			t.Fatalf("refusal reason lost: %s", w.Body.String())
		}
	})

	t.Run("DELETE is a 204", func(t *testing.T) {
		var gotID string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			deleteContactFn: func(_ context.Context, id string) ([]byte, error) { gotID = id; return nil, nil },
		})
		w := call(h, http.MethodDelete, "/team-schedule/paging-contacts/"+userID, userID, "", true, h.DeletePagingContact)
		assertStatus(t, w, http.StatusNoContent)
		if gotID != userID {
			t.Fatalf("deleted %q", gotID)
		}
		assertStatus(t, call(h, http.MethodDelete, "/x", "nope", "", true, h.DeletePagingContact), http.StatusBadRequest)
		assertStatus(t, call(h, http.MethodDelete, "/x", userID, "", false, h.DeletePagingContact), http.StatusUnauthorized)
	})

	t.Run("test call is a 202, 409 and 429 are kept", func(t *testing.T) {
		var upstream error
		h := NewScheduleHandler(&mockEntityScheduleClient{
			testContactFn: func(context.Context, string) ([]byte, error) {
				if upstream != nil {
					return nil, upstream
				}
				return []byte(`{"lastTestStatus":"pending"}`), nil
			},
		})
		test := func() *httptest.ResponseRecorder {
			return call(h, http.MethodPost, "/team-schedule/paging-contacts/"+userID+"/test", userID, "", true, h.TestPagingContact)
		}
		w := test()
		assertStatus(t, w, http.StatusAccepted)
		if !strings.Contains(w.Body.String(), `"pending"`) {
			t.Fatalf("body = %s", w.Body.String())
		}

		upstream = &apierror.Error{StatusCode: http.StatusConflict, Body: `{"message":"this person has no paging number"}`}
		w = test()
		assertStatus(t, w, http.StatusConflict)
		if !strings.Contains(w.Body.String(), "this person has no paging number") {
			t.Fatalf("409 reason lost: %s", w.Body.String())
		}

		upstream = &apierror.Error{StatusCode: http.StatusTooManyRequests, Body: `{"message":"internal limiter"}`}
		w = test()
		assertStatus(t, w, http.StatusTooManyRequests)
		if !strings.Contains(w.Body.String(), "less than 2 minutes ago") || strings.Contains(w.Body.String(), "internal") {
			t.Fatalf("429 body = %s", w.Body.String())
		}

		upstream = &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"message":"twilio: secret"}`}
		w = test()
		assertStatus(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "twilio") {
			t.Fatalf("upstream detail leaked: %s", w.Body.String())
		}
	})
}

func TestDismissScheduleEngagementAllocation(t *testing.T) {
	const path = "/team-schedule/engagement-allocations/dismiss"
	const payload = `{"id":"00000000-0000-0000-0000-000000000001","teamKey":"castor","from":"2026-11-02","to":"2026-11-03"}`

	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.DismissScheduleEngagementAllocation(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload)))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects a body that is not JSON", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.DismissScheduleEngagementAllocation(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(`nope`))))
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("forwards the body and returns the answer as sent", func(t *testing.T) {
		var captured []byte
		h := NewScheduleHandler(&mockEntityScheduleClient{dismissFn: func(_ context.Context, body []byte) ([]byte, error) {
			captured = body
			return []byte(`{"days":2}`), nil
		}})
		w := httptest.NewRecorder()
		h.DismissScheduleEngagementAllocation(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))))
		assertStatus(t, w, http.StatusOK)
		if string(captured) != payload {
			t.Fatalf("forwarded %s, want the body unchanged", captured)
		}
		if strings.TrimSpace(w.Body.String()) != `{"days":2}` {
			t.Fatalf("body = %s", w.Body.String())
		}
	})

	t.Run("shows the entity service's refusal", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{dismissFn: func(context.Context, []byte) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"message":"that engineer is not on castor, so their rota is not yours to change"}`}
		}})
		w := httptest.NewRecorder()
		h.DismissScheduleEngagementAllocation(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))))
		assertStatus(t, w, http.StatusForbidden)
	})
}
