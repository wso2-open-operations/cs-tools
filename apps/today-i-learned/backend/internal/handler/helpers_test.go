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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/entry"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/store"
)

const webappClientID = "one-wso2-webapp"

var (
	jane = &middleware.User{Subject: "u-jane", Email: "jane@example.com", DisplayName: "Jane Doe", ClientIDs: []string{webappClientID}}
	sam  = &middleware.User{Subject: "u-sam", Email: "sam@example.com", DisplayName: "Sam", ClientIDs: []string{webappClientID}}
	mod  = &middleware.User{Subject: "u-mod", Email: "mod@example.com", DisplayName: "Mod", IsModerator: true, ClientIDs: []string{webappClientID}}
	// janeViaOtherApp is Jane signed in through some other application: a
	// valid token, issued to a different client.
	janeViaOtherApp = &middleware.User{Subject: "u-jane", Email: "jane@example.com", DisplayName: "Jane Doe", ClientIDs: []string{"another-app"}}
)

// fakeStore keeps entries in memory. beforeUpdate and beforeClaim, when set,
// run just before an update or a notification claim is applied.
type fakeStore struct {
	mu           sync.Mutex
	entries      map[string]entry.Entry
	order        []string
	seq          int
	beforeUpdate func(id string)
	beforeClaim  func(id string)
	lastQuery    store.Query
	fail         error
}

func newFakeStore() *fakeStore { return &fakeStore{entries: map[string]entry.Entry{}} }

func (s *fakeStore) Create(_ context.Context, a store.Author, c entry.Content) (entry.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return entry.Entry{}, s.fail
	}
	s.seq++
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", s.seq)
	e := entry.Entry{ID: id, Title: c.Title, Who: a.Name, Where: c.Where, WhereDetail: c.WhereDetail, What: c.HTML,
		SubmittedByEmail: a.Email, CreatedAt: time.Date(2026, 10, 1, 0, 0, s.seq, 0, time.UTC)}
	s.entries[id] = e
	s.order = append(s.order, id)
	return e, nil
}

func (s *fakeStore) Get(_ context.Context, id string) (entry.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		return entry.Entry{}, store.ErrNotFound
	}
	return e, nil
}

func (s *fakeStore) Update(_ context.Context, id string, c entry.Content) (entry.Entry, error) {
	if s.beforeUpdate != nil {
		s.beforeUpdate(id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		return entry.Entry{}, store.ErrNotFound
	}
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	e.Title, e.Where, e.WhereDetail, e.What, e.UpdatedAt = c.Title, c.Where, c.WhereDetail, c.HTML, &now
	s.entries[id] = e
	return e, nil
}

func (s *fakeStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[id]; !ok {
		return store.ErrNotFound
	}
	delete(s.entries, id)
	return nil
}

func (s *fakeStore) List(_ context.Context, q store.Query) (store.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastQuery = q
	items := []entry.Entry{}
	for i := len(s.order) - 1; i >= 0; i-- {
		if e, ok := s.entries[s.order[i]]; ok && (q.AuthorEmail == "" || e.SubmittedByEmail == q.AuthorEmail) {
			items = append(items, e)
		}
	}
	total := len(items)
	if q.Offset < len(items) {
		items = items[q.Offset:min(len(items), q.Offset+q.Limit)]
	} else {
		items = []entry.Entry{}
	}
	return store.Page{Items: items, Total: total}, nil
}

func (s *fakeStore) ClaimNotification(_ context.Context, id string) (time.Time, bool, error) {
	if s.beforeClaim != nil {
		s.beforeClaim(id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return time.Time{}, false, s.fail
	}
	e, ok := s.entries[id]
	if !ok || e.NotifiedAt != nil {
		return time.Time{}, false, nil
	}
	at := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	e.NotifiedAt = &at
	s.entries[id] = e
	return at, true, nil
}

func (s *fakeStore) ReleaseNotification(_ context.Context, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[id]; ok && e.NotifiedAt != nil && e.NotifiedAt.Equal(at) {
		e.NotifiedAt = nil
		s.entries[id] = e
	}
	return nil
}

type notice struct {
	entry    entry.Entry
	entryURL string
}

// fakeNotifier records what it's asked to announce. fail, when set, is
// returned as Novera not accepting the announcement.
type fakeNotifier struct {
	mu   sync.Mutex
	sent []notice
	fail error
}

func (n *fakeNotifier) Notify(_ context.Context, e entry.Entry, entryURL string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, notice{e, entryURL})
	return n.fail
}

type testAPI struct {
	store    *fakeStore
	notifier *fakeNotifier
	mux      *http.ServeMux
}

func newTestAPI(t *testing.T, change ...func(*Config)) *testAPI {
	t.Helper()
	api := &testAPI{store: newFakeStore(), notifier: &fakeNotifier{}}
	cfg := Config{Store: api.store, Notifier: api.notifier, WebappClientID: webappClientID, WebappBaseURL: "https://one.example/"}
	for _, c := range change {
		c(&cfg)
	}
	h := New(cfg)
	api.mux = http.NewServeMux()
	api.mux.HandleFunc("GET /user-info", h.UserInfo)
	api.mux.HandleFunc("GET /settings", h.Settings)
	api.mux.HandleFunc("GET /submissions", h.ListEntries)
	api.mux.HandleFunc("POST /submissions", h.CreateEntry)
	api.mux.HandleFunc("GET /submissions/{id}", h.GetEntry)
	api.mux.HandleFunc("PUT /submissions/{id}", h.UpdateEntry)
	api.mux.HandleFunc("DELETE /submissions/{id}", h.DeleteEntry)
	api.mux.HandleFunc("POST /submissions/{id}/notify", h.NotifyEntry)
	return api
}

// do sends a request as user (nil: no signed-in user) and returns the
// response.
func (a *testAPI) do(t *testing.T, user *middleware.User, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return record(a.mux, newRequest(t, user, method, path, body))
}

func newRequest(t *testing.T, user *middleware.User, method, path, body string) *http.Request {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if user != nil {
		req = req.WithContext(middleware.WithUser(req.Context(), user))
	}
	return req
}

func record(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func (a *testAPI) create(t *testing.T, user *middleware.User, body string) entry.Entry {
	t.Helper()
	w := a.do(t, user, http.MethodPost, "/submissions", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status %d: %s", w.Code, w.Body)
	}
	return decodeJSON[entry.Entry](t, w)
}

func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", w.Body, err)
	}
	return v
}

// assertError checks the status and that the body is {"message": want}.
func assertError(t *testing.T, w *httptest.ResponseRecorder, status int, want string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", w.Code, status, w.Body)
	}
	body := decodeJSON[map[string]any](t, w)
	if len(body) != 1 || body["message"] != want {
		t.Fatalf("body = %v, want {\"message\": %q}", body, want)
	}
}

const validBody = `{"title":"T","who":"Jane Doe","where":"Internal","what":"<p>Learned X.</p>"}`
