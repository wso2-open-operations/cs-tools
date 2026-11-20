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
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

const (
	testLogItemID         = "9d48f18f-0ae3-466b-b0ab-fdd14038864b"
	testLogCaseID         = "22222222-2222-2222-2222-222222222222"
	testLogCaseNumber     = "CS0012345"
	testLogCreateResponse = `{"message":"Case created successfully.","case":{"id":"` + testLogCaseID + `","number":"` + testLogCaseNumber + `"}}`
	testLogOwner          = "example-org"
	testLogRepo           = "pipelines"
	testLogTeam           = "sre"
)

type dispatchCall struct {
	orgName, owner, repo, eventType string
	payload                         map[string]any
	ctxErr                          error
}

// mockPipelineDispatcher returns errs in order, then nil. A non-nil release
// holds every call until it is closed.
type mockPipelineDispatcher struct {
	mu      sync.Mutex
	calls   []dispatchCall
	errs    []error
	release chan struct{}
}

func (m *mockPipelineDispatcher) DispatchRepositoryEvent(ctx context.Context, orgName, owner, repoName, eventType string, payload map[string]any) error {
	if m.release != nil {
		<-m.release
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, dispatchCall{orgName, owner, repoName, eventType, payload, ctx.Err()})
	if n := len(m.calls); n <= len(m.errs) {
		return m.errs[n-1]
	}
	return nil
}

// noDispatchRetryWait makes retries immediate.
func noDispatchRetryWait(t *testing.T) {
	t.Helper()
	prev := dispatchRetryDelays
	dispatchRetryDelays = []time.Duration{0, 0}
	t.Cleanup(func() { dispatchRetryDelays = prev })
}

func logRequestBody(itemID string, answers map[string]string) string {
	type variable struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}
	vars := []variable{}
	for id, v := range answers {
		vars = append(vars, variable{id, v})
	}
	b, _ := json.Marshal(map[string]any{
		"type":          "service_request",
		"projectId":     "proj-1",
		"catalogId":     "63cafc50-1ba2-0210-609e-8666624bcb38",
		"catalogItemId": itemID,
		"variables":     vars,
	})
	return string(b)
}

func completeLogRequestAnswers() map[string]string {
	return map[string]string{
		clusterQuestionID:     "choreo-dev-cdp-log-collector",
		environmentQuestionID: "production",
		namespaceQuestionID:   "dp-a",
		podNameQuestionID:     "orders-api",
		startTimeQuestionID:   "2026-11-12T08:00:00.000Z",
		endTimeQuestionID:     "2026-11-12T09:00:00.000Z",
		namedEmailsQuestionID: "one@example.com, two@example.com",
		componentQuestionID:   "orders",
		searchTextQuestionID:  " ",
	}
}

type logCollectionTest struct {
	h     *CaseHandler
	disp  *mockPipelineDispatcher
	notes []string
}

func newLogCollectionTest(t *testing.T, disp *mockPipelineDispatcher, createErr error) *logCollectionTest {
	t.Helper()
	lt := &logCollectionTest{disp: disp}
	var mu sync.Mutex
	client := &mockEntityCaseClient{
		createCaseFn: func(context.Context, []byte) ([]byte, error) {
			if createErr != nil {
				return nil, createErr
			}
			return []byte(testLogCreateResponse), nil
		},
		createCaseCommentFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
			var note struct{ Type, Content string }
			if err := json.Unmarshal(body, &note); err != nil || note.Type != "work_note" || caseID != testLogCaseID {
				t.Errorf("work note = %s on case %s, want a work_note on %s", body, caseID, testLogCaseID)
			}
			mu.Lock()
			lt.notes = append(lt.notes, note.Content)
			mu.Unlock()
			return []byte(`{}`), nil
		},
	}
	lt.h = NewCaseHandler(client).WithLogCollection(disp, LogCollectionConfig{
		CatalogItemID: testLogItemID, Owner: testLogOwner, Repo: testLogRepo, Team: testLogTeam,
	})
	return lt
}

// create calls CreateCase and waits for the background start to finish.
func (lt *logCollectionTest) create(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(body)))
	w := httptest.NewRecorder()
	lt.h.CreateCase(w, r)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lt.h.WaitForLogCollection(ctx)
	return w
}

func TestCreateCase_LogCollection(t *testing.T) {
	noDispatchRetryWait(t)

	t.Run("starts the pipeline for a Component Log Request and notes it on the case", func(t *testing.T) {
		lt := newLogCollectionTest(t, &mockPipelineDispatcher{}, nil)
		w := lt.create(t, logRequestBody(testLogItemID, completeLogRequestAnswers()))

		assertStatus(t, w, http.StatusCreated)
		if len(lt.disp.calls) != 1 {
			t.Fatalf("dispatch calls = %d, want 1", len(lt.disp.calls))
		}
		c := lt.disp.calls[0]
		if c.orgName != testLogOwner || c.owner != testLogOwner || c.repo != testLogRepo || c.eventType != "request_logs" {
			t.Errorf("dispatch target = %+v", c)
		}
		want := map[string]any{
			"case_id":      testLogCaseID,
			"request_id":   testLogCaseNumber,
			"requested_by": testUser.Email,
			"team":         testLogTeam,
			"cluster":      "choreo-dev-cdp-log-collector",
			"pod_name":     "orders-api",
			"start_time":   "2026-11-12T08:00:00.000Z",
			"end_time":     "2026-11-12T09:00:00.000Z",
			"named_emails": "one@example.com, two@example.com",
			"options": map[string]any{
				"environment": "production",
				"namespace":   "dp-a",
				"component":   "orders",
			},
		}
		if !reflect.DeepEqual(c.payload, want) {
			t.Errorf("payload = %v\nwant      %v", c.payload, want)
		}
		if len(c.payload) > 10 {
			t.Errorf("payload has %d top-level properties; GitHub allows 10", len(c.payload))
		}
		if len(lt.notes) != 1 || !strings.HasPrefix(lt.notes[0], "Log collection started.") {
			t.Errorf("work notes = %q", lt.notes)
		}
	})

	t.Run("answers before the dispatch, which keeps the request's values but not its cancellation", func(t *testing.T) {
		disp := &mockPipelineDispatcher{release: make(chan struct{})}
		lt := newLogCollectionTest(t, disp, nil)
		ctx, cancelRequest := context.WithCancel(context.Background())
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(logRequestBody(testLogItemID, completeLogRequestAnswers()))).WithContext(ctx))
		w := httptest.NewRecorder()

		lt.h.CreateCase(w, r) // returns while the dispatch is still held
		assertStatus(t, w, http.StatusCreated)
		cancelRequest()
		close(disp.release)
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		lt.h.WaitForLogCollection(waitCtx)

		if len(disp.calls) != 1 || disp.calls[0].ctxErr != nil {
			t.Fatalf("calls = %+v, want one with a live context", disp.calls)
		}
	})

	t.Run("leaves other service requests alone", func(t *testing.T) {
		lt := newLogCollectionTest(t, &mockPipelineDispatcher{}, nil)
		w := lt.create(t, logRequestBody("33333333-3333-3333-3333-333333333333", completeLogRequestAnswers()))

		assertStatus(t, w, http.StatusCreated)
		if len(lt.disp.calls) != 0 || len(lt.notes) != 0 {
			t.Errorf("calls = %d, notes = %q; want none", len(lt.disp.calls), lt.notes)
		}
	})

	t.Run("leaves a case alone even if it names the item", func(t *testing.T) {
		lt := newLogCollectionTest(t, &mockPipelineDispatcher{}, nil)
		body := strings.Replace(logRequestBody(testLogItemID, completeLogRequestAnswers()), `"service_request"`, `"case"`, 1)
		lt.create(t, body)

		if len(lt.disp.calls) != 0 {
			t.Errorf("dispatch calls = %d, want 0", len(lt.disp.calls))
		}
	})

	t.Run("does nothing when the create fails", func(t *testing.T) {
		lt := newLogCollectionTest(t, &mockPipelineDispatcher{}, &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"message":"forbidden"}`})
		w := lt.create(t, logRequestBody(testLogItemID, completeLogRequestAnswers()))

		assertStatus(t, w, http.StatusForbidden)
		if len(lt.disp.calls) != 0 || len(lt.notes) != 0 {
			t.Errorf("calls = %d, notes = %q; want none", len(lt.disp.calls), lt.notes)
		}
	})

	t.Run("does nothing when not configured", func(t *testing.T) {
		client := &mockEntityCaseClient{
			createCaseFn: func(context.Context, []byte) ([]byte, error) { return []byte(testLogCreateResponse), nil },
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(logRequestBody(testLogItemID, completeLogRequestAnswers()))))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		h.WaitForLogCollection(context.Background())

		assertStatus(t, w, http.StatusCreated)
	})

	t.Run("does not dispatch with required answers missing, and says which", func(t *testing.T) {
		lt := newLogCollectionTest(t, &mockPipelineDispatcher{}, nil)
		answers := completeLogRequestAnswers()
		delete(answers, clusterQuestionID)
		answers[podNameQuestionID] = "  "
		lt.create(t, logRequestBody(testLogItemID, answers))

		if len(lt.disp.calls) != 0 {
			t.Errorf("dispatch calls = %d, want 0", len(lt.disp.calls))
		}
		if len(lt.notes) != 1 || !strings.Contains(lt.notes[0], "Cluster, Pod Name") {
			t.Errorf("work notes = %q, want one naming Cluster and Pod Name", lt.notes)
		}
	})

	t.Run("still answers 201 when the dispatch fails, and notes the failure", func(t *testing.T) {
		forbidden := &apierror.Error{StatusCode: http.StatusForbidden, Body: "token detail"}
		lt := newLogCollectionTest(t, &mockPipelineDispatcher{errs: []error{forbidden}}, nil)
		w := lt.create(t, logRequestBody(testLogItemID, completeLogRequestAnswers()))

		assertStatus(t, w, http.StatusCreated)
		if len(lt.notes) != 1 || !strings.Contains(lt.notes[0], "could not be started (the Engineering Entity Service answered 403)") {
			t.Errorf("work notes = %q", lt.notes)
		}
		if strings.Contains(lt.notes[0], "token detail") {
			t.Error("the work note carries the upstream response body")
		}
	})
}

func TestDispatchWithRetry(t *testing.T) {
	noDispatchRetryWait(t)
	status := func(code int) error { return &apierror.Error{StatusCode: code} }
	network := errors.New("dial tcp: connection refused")

	for _, tc := range []struct {
		name      string
		errs      []error
		wantCalls int
		wantErr   bool
	}{
		{"succeeds first time", nil, 1, false},
		{"retries a 5xx", []error{status(http.StatusBadGateway)}, 2, false},
		{"retries a 429", []error{status(http.StatusTooManyRequests)}, 2, false},
		{"retries a network error", []error{network, network}, 3, false},
		{"gives up after three attempts", []error{status(500), status(503), status(502), nil}, 3, true},
		{"does not retry a 400", []error{status(http.StatusBadRequest)}, 1, true},
		{"does not retry a 403", []error{status(http.StatusForbidden)}, 1, true},
		{"does not retry a 404", []error{status(http.StatusNotFound)}, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			disp := &mockPipelineDispatcher{errs: tc.errs}
			failures := 0
			err := dispatchWithRetry(context.Background(), func(ctx context.Context) error {
				return disp.DispatchRepositoryEvent(ctx, "o", "o", "r", "e", nil)
			}, func(int, error) { failures++ })

			if len(disp.calls) != tc.wantCalls {
				t.Errorf("calls = %d, want %d", len(disp.calls), tc.wantCalls)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, want error %v", err, tc.wantErr)
			}
			if wantFailures := min(len(tc.errs), tc.wantCalls); failures != wantFailures {
				t.Errorf("failures reported = %d, want %d", failures, wantFailures)
			}
		})
	}

	t.Run("stops waiting when the context ends", func(t *testing.T) {
		dispatchRetryDelays = []time.Duration{time.Hour, time.Hour}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		disp := &mockPipelineDispatcher{errs: []error{status(503), status(503), status(503)}}
		start := time.Now()
		err := dispatchWithRetry(ctx, func(ctx context.Context) error {
			return disp.DispatchRepositoryEvent(ctx, "o", "o", "r", "e", nil)
		}, func(int, error) {})

		if err == nil || len(disp.calls) != 1 || time.Since(start) > 5*time.Second {
			t.Errorf("err = %v, calls = %d after %v; want the first error, at once", err, len(disp.calls), time.Since(start))
		}
	})
}
