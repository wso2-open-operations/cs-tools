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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/pinecone"
)

const (
	uuidA = "11111111-1111-1111-1111-111111111111"
	uuidB = "22222222-2222-2222-2222-222222222222"
	uuidC = "33333333-3333-3333-3333-333333333333"
	// uuidMissing is a well-formed id no fake holds.
	uuidMissing = "99999999-9999-9999-9999-999999999999"
	uuidD       = "44444444-4444-4444-4444-444444444444"
	kbOne       = "aaaaaaaa-0000-4000-8000-000000000001"
	kbOff       = "aaaaaaaa-0000-4000-8000-0000000000ff"
	kbUnknown   = "aaaaaaaa-0000-4000-8000-000000000404"
)

// fakeEntity records the calls it receives and serves canned answers.
type fakeEntity struct {
	mu        sync.Mutex
	kbs       []entity.KnowledgeBase
	kbsErr    error
	articles  map[string]entity.KBArticle
	getErr    map[string]error
	search    entity.SearchResponse
	searchErr error

	// kbGate, when set, blocks ListKnowledgeBases until it is closed.
	kbGate chan struct{}

	getIDs      []string
	kbCalls     int
	searchCalls []entity.SearchRequest
}

func (f *fakeEntity) ListKnowledgeBases(context.Context) ([]entity.KnowledgeBase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kbCalls++
	if f.kbGate != nil {
		gate := f.kbGate
		f.mu.Unlock()
		<-gate
		f.mu.Lock()
	}
	if f.kbs == nil && f.kbsErr == nil {
		// Default: the one knowledge base every test article lives in is active.
		return []entity.KnowledgeBase{{ID: kbOne, Title: "One", Active: true}}, nil
	}
	return f.kbs, f.kbsErr
}

func (f *fakeEntity) SearchKBArticles(_ context.Context, req entity.SearchRequest) (entity.SearchResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searchCalls = append(f.searchCalls, req)
	return f.search, f.searchErr
}

func (f *fakeEntity) GetKBArticle(_ context.Context, id string) (entity.KBArticle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getIDs = append(f.getIDs, id)
	// Mirrors real entity-service: a malformed id is 400, a missing article is 404.
	if _, ok := normalizeID(id); !ok {
		return entity.KBArticle{}, &apierror.Error{StatusCode: http.StatusBadRequest, Body: "invalid article id"}
	}
	if err := f.getErr[id]; err != nil {
		return entity.KBArticle{}, err
	}
	a, ok := f.articles[id]
	if !ok {
		return entity.KBArticle{}, &apierror.Error{StatusCode: http.StatusNotFound, Body: "article not found"}
	}
	return a, nil
}

type fakeSimilarity struct {
	matches []pinecone.Match
	err     error
}

func (f fakeSimilarity) Similar(context.Context, string, int) ([]pinecone.Match, error) {
	return f.matches, f.err
}

func article(id, state string) entity.KBArticle {
	pub := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return entity.KBArticle{
		ID: id, KnowledgeBaseID: kbOne, Title: "Title " + id, Body: "<p>body</p>", State: state,
		CreatedOn: pub, UpdatedOn: pub, PublishedOn: &pub,
	}
}

// newMux wires the handlers on the same patterns main.go registers.
func newMux(e Entity, s Similarity) *http.ServeMux {
	kb := NewKBHandler(e, s)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /public/knowledge-bases", kb.ListKnowledgeBases)
	mux.HandleFunc("POST /public/kb-articles/search", kb.SearchArticles)
	mux.HandleFunc("GET /public/kb-articles/{id}", kb.GetArticle)
	mux.HandleFunc("GET /public/kb-articles/{id}/related", kb.RelatedArticles)
	return mux
}

func do(mux http.Handler, method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		id   string
		want string // "" means rejected
	}{
		{uuidA, uuidA},
		{"ABCDEF01-2345-6789-ABCD-EF0123456789", "abcdef01-2345-6789-abcd-ef0123456789"},
		{"abcdef01-2345-6789-abcd-ef0123456789", "abcdef01-2345-6789-abcd-ef0123456789"},
		// The old slug form is gone.
		{"abc_DEF-123", ""},
		{"kb-1", ""},
		{strings.Repeat("a", 36), ""},
		{"", ""},
		{"..", ""},
		{"a/b", ""},
		{"a%2Fb", ""},
		{"..%2F..%2Fcases", ""},
		{"a?x=1", ""},
		{"a#frag", ""},
		{"a b", ""},
		{"a\nb", ""},
		{"idé", ""},
		{uuidA + "\n", ""},                       // trailing newline must not slip past $
		{" " + uuidA, ""},                        // leading space
		{"{" + uuidA + "}", ""},                  // braces form
		{strings.ReplaceAll(uuidA, "-", ""), ""}, // 32 hex, no dashes
		{"urn:uuid:" + uuidA, ""},
		{"1111111g-1111-1111-1111-111111111111", ""}, // non-hex
		{uuidA + "0", ""},
	}
	for _, tt := range tests {
		got, ok := normalizeID(tt.id)
		if ok != (tt.want != "") || got != tt.want {
			t.Errorf("normalizeID(%q) = %q, %v; want %q", tt.id, got, ok, tt.want)
		}
	}
}

func TestGetArticleRejectsBadIDsBeforeUpstream(t *testing.T) {
	for _, id := range []string{"a%2F..%2Fcases", "a%3Fx%3D1", "a%23b", "a%2e%2e", strings.Repeat("a", 65), "slug-id", "not-a-uuid", "%7B" + uuidA + "%7D", strings.ReplaceAll(uuidA, "-", "")} {
		t.Run(id, func(t *testing.T) {
			f := &fakeEntity{}
			rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+id, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
			}
			if len(f.getIDs) != 0 {
				t.Fatalf("upstream was called with %v for a rejected id", f.getIDs)
			}
		})
	}
}

func TestGetArticlePublishedOnlyAndDTOWhitelist(t *testing.T) {
	leaky := article(uuidA, "published")
	leaky.Number = ptr("KB0001")
	tests := []struct {
		name       string
		article    entity.KBArticle
		upstream   error
		wantStatus int
	}{
		{"published", leaky, nil, http.StatusOK},
		{"draft is 404", article(uuidA, "draft"), nil, http.StatusNotFound},
		{"pending_review is 404", article(uuidA, "pending_review"), nil, http.StatusNotFound},
		{"retired is 404", article(uuidA, "retired"), nil, http.StatusNotFound},
		{"empty state is 404", article(uuidA, ""), nil, http.StatusNotFound},
		{"upstream 404", entity.KBArticle{}, &apierror.Error{StatusCode: 404}, http.StatusNotFound},
		{"upstream 400 (malformed id) is 404, not relayed", entity.KBArticle{}, &apierror.Error{StatusCode: 400, Body: "secret detail"}, http.StatusNotFound},
		{"upstream 403 is 502", entity.KBArticle{}, &apierror.Error{StatusCode: 403, Body: "secret detail"}, http.StatusBadGateway},
		{"upstream 500 is 502", entity.KBArticle{}, &apierror.Error{StatusCode: 500, Body: "stack trace"}, http.StatusBadGateway},
		{"upstream timeout is 504", entity.KBArticle{}, context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"transport error is 502", entity.KBArticle{}, errors.New("dial tcp 10.0.0.1: refused"), http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: tt.article}, getErr: map[string]error{}}
			if tt.upstream != nil {
				f.getErr[uuidA] = tt.upstream
			}
			rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA, "")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			body := rec.Body.String()
			for _, leak := range []string{"secret detail", "stack trace", "10.0.0.1", "refused"} {
				if strings.Contains(body, leak) {
					t.Errorf("response leaks upstream detail %q: %s", leak, body)
				}
			}
			if tt.wantStatus != http.StatusOK {
				var e errorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Message == "" {
					t.Errorf("want {message} envelope, got %s", body)
				}
				return
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{"id": true, "knowledgeBaseId": true, "title": true, "body": true, "number": true,
				"createdOn": true, "updatedOn": true, "publishedOn": true}
			for k := range got {
				if !want[k] {
					t.Errorf("unexpected field %q in public article", k)
				}
			}
			for k := range want {
				if _, ok := got[k]; !ok {
					t.Errorf("missing field %q", k)
				}
			}
		})
	}
}

// TestArticleDTODropsInternalFields feeds the real entity-service JSON shape
// (with every internal field populated) through the client types and checks the
// served JSON never mentions them.
func TestArticleDTODropsInternalFields(t *testing.T) {
	const upstream = `{"id":"` + uuidA + `","knowledgeBaseId":"` + kbOne + `","title":"T","body":"<p>b</p>","state":"published",
		"authorId":"author-secret","revisedById":"reviser-secret","sourceCaseId":"case-secret","rejectionComment":"reject-secret",
		"updatedBy":"updater-secret","baseVersionId":"base-secret","latest":true,"createdOn":"2026-01-01T00:00:00Z",
		"updatedOn":"2026-01-02T00:00:00Z","publishedOn":"2026-01-03T00:00:00Z","retiredOn":null}`
	var a entity.KBArticle
	if err := json.Unmarshal([]byte(upstream), &a); err != nil {
		t.Fatal(err)
	}
	f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: a}}
	rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	for _, leak := range []string{"secret", "authorId", "revisedById", "sourceCaseId", "rejectionComment", "updatedBy", "baseVersionId", "latest", "state"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("public article contains %q: %s", leak, rec.Body)
		}
	}
}

func TestSearchForcesPublishedAndFiltersResults(t *testing.T) {
	f := &fakeEntity{search: entity.SearchResponse{
		Articles: []entity.KBArticle{article(uuidA, "published"), article(uuidB, "draft"), article(uuidC, "retired")},
		Total:    3, Limit: 20, Offset: 0,
	}}
	// "states" in the body must be rejected, not honoured.
	rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{"states":["draft"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("states in body: status = %d, want 400", rec.Code)
	}
	if len(f.searchCalls) != 0 {
		t.Fatal("upstream called for a rejected request")
	}

	rec = do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{"knowledgeBaseId":"`+kbOne+`","searchQuery":"  oauth  ","limit":10,"offset":5}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if len(f.searchCalls) != 1 {
		t.Fatalf("search calls = %d", len(f.searchCalls))
	}
	got := f.searchCalls[0]
	if len(got.States) != 1 || got.States[0] != "published" {
		t.Errorf("States = %v, want [published]", got.States)
	}
	if got.KnowledgeBaseID != kbOne || got.SearchQuery != "oauth" || got.Pagination != (entity.Pagination{Limit: 10, Offset: 5}) {
		t.Errorf("upstream request = %+v", got)
	}
	var resp struct {
		Articles []map[string]any `json:"articles"`
		Total    int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Articles) != 1 || resp.Articles[0]["id"] != uuidA {
		t.Errorf("non-published articles leaked through: %s", rec.Body)
	}
}

func TestSearchUpstreamRequestHasNoSortBy(t *testing.T) {
	// entity-service decodes strictly: a sortBy key would be a 400 upstream.
	b, _ := json.Marshal(entity.SearchRequest{States: []string{"published"}})
	if strings.Contains(string(b), "sortBy") {
		t.Fatalf("upstream search body carries sortBy: %s", b)
	}
}

func TestSearchInputBounds(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"empty body", ``, 200},
		{"empty object", `{}`, 200},
		{"limit default", `{"limit":0}`, 200},
		{"limit max", `{"limit":50}`, 200},
		{"limit over", `{"limit":51}`, 400},
		{"limit negative", `{"limit":-1}`, 400},
		{"offset max", `{"offset":10000}`, 200},
		{"offset over", `{"offset":10001}`, 400},
		{"offset negative", `{"offset":-1}`, 400},
		{"query at cap", `{"searchQuery":"` + strings.Repeat("a", 200) + `"}`, 200},
		{"query over cap", `{"searchQuery":"` + strings.Repeat("a", 201) + `"}`, 400},
		{"query multibyte at cap", `{"searchQuery":"` + strings.Repeat("é", 200) + `"}`, 200},
		{"query control char", `{"searchQuery":"a\u0000b"}`, 400},
		{"kb id ok", `{"knowledgeBaseId":"` + kbOne + `"}`, 200},
		{"kb id upper case is normalised", `{"knowledgeBaseId":"` + strings.ToUpper(kbOne) + `"}`, 200},
		{"kb id slug rejected", `{"knowledgeBaseId":"kb-1"}`, 400},
		{"kb id valid but unknown", `{"knowledgeBaseId":"` + uuidA + `"}`, 404},
		{"kb id traversal", `{"knowledgeBaseId":"../x"}`, 400},
		{"kb id too long", `{"knowledgeBaseId":"` + strings.Repeat("a", 65) + `"}`, 400},
		{"kb id malformed uuid", `{"knowledgeBaseId":"` + uuidA + `0"}`, 400},
		{"sortBy recent", `{"sortBy":"recent"}`, 200},
		{"sortBy mostViewed unsupported", `{"sortBy":"mostViewed"}`, 400},
		{"sortBy topRated unsupported", `{"sortBy":"topRated"}`, 400},
		{"sortBy injection", `{"sortBy":"created_on; drop table"}`, 400},
		{"malformed json", `{"limit":`, 400},
		{"not an object", `[1,2]`, 400},
		{"wrong type", `{"limit":"ten"}`, 400},
		{"unknown field", `{"authorId":"x"}`, 400},
		{"trailing data", `{} {}`, 400},
		{"body too large", `{"searchQuery":"` + strings.Repeat("a", 20<<10) + `"}`, 413},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeEntity{}
			rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != 200 && len(f.searchCalls) != 0 {
				t.Error("upstream called for a rejected request")
			}
		})
	}
}

func TestSearchDefaultLimit(t *testing.T) {
	f := &fakeEntity{}
	do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{}`)
	if got := f.searchCalls[0].Pagination.Limit; got != defaultLimit {
		t.Errorf("limit = %d, want %d", got, defaultLimit)
	}
}

func TestSearchUpstreamErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"upstream 400 is 502, not relayed", &apierror.Error{StatusCode: 400, Body: "unknown field sortBy"}, 502},
		{"upstream 404 is 502 on search", &apierror.Error{StatusCode: 404}, 502},
		{"upstream 503", &apierror.Error{StatusCode: 503}, 502},
		{"timeout", context.DeadlineExceeded, 504},
		{"wrapped timeout", errors.Join(errors.New("x"), context.DeadlineExceeded), 504},
		{"other", errors.New("boom"), 502},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeEntity{searchErr: tt.err}
			rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{}`)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if strings.Contains(rec.Body.String(), "sortBy") || strings.Contains(rec.Body.String(), "boom") {
				t.Errorf("upstream detail relayed: %s", rec.Body)
			}
		})
	}
}

func TestListKnowledgeBasesOnlyActive(t *testing.T) {
	f := &fakeEntity{kbs: []entity.KnowledgeBase{
		{ID: kbOne, Title: "One", Active: true},
		{ID: "bbbbbbbb-0000-4000-8000-000000000002", Title: "Hidden", Active: false},
		{ID: "bad/id", Title: "Weird", Active: true},
		{ID: "kb-slug", Title: "Slug", Active: true},
	}}
	rec := do(newMux(f, nil), http.MethodGet, "/public/knowledge-bases", "")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var got listKnowledgeBasesDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.KnowledgeBases) != 1 || got.KnowledgeBases[0].ID != kbOne || got.KnowledgeBases[0].Title != "One" {
		t.Errorf("got %+v", got.KnowledgeBases)
	}
	// Whitelist: id and title only, nothing else (no active flag, no timestamps).
	var raw struct {
		KnowledgeBases []map[string]any `json:"knowledgeBases"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	if len(raw.KnowledgeBases) != 1 || len(raw.KnowledgeBases[0]) != 2 {
		t.Errorf("knowledge base DTO is not id+title only: %s", rec.Body)
	}

	f = &fakeEntity{kbsErr: &apierror.Error{StatusCode: 500, Body: "pg down"}}
	rec = do(newMux(f, nil), http.MethodGet, "/public/knowledge-bases", "")
	if rec.Code != 502 || strings.Contains(rec.Body.String(), "pg down") {
		t.Errorf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestRelatedRequiresPublishedSource(t *testing.T) {
	for _, state := range []string{"draft", "pending_review", "retired"} {
		f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: article(uuidA, state)}}
		sim := fakeSimilarity{matches: []pinecone.Match{{ID: uuidB, Score: 0.9}}}
		rec := do(newMux(f, sim), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", state, rec.Code)
		}
		if len(f.getIDs) != 1 || len(f.searchCalls) != 0 {
			t.Errorf("%s: lookups after a rejected source: get=%v search=%d", state, f.getIDs, len(f.searchCalls))
		}
	}
	rec := do(newMux(&fakeEntity{}, nil), http.MethodGet, "/public/kb-articles/../related", "")
	if rec.Code == http.StatusOK {
		t.Error("bad id should not succeed")
	}
	rec = do(newMux(&fakeEntity{}, nil), http.MethodGet, "/public/kb-articles/a%2Fb/related", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("encoded slash id: status = %d, want 400", rec.Code)
	}
	rec = do(newMux(&fakeEntity{}, nil), http.MethodGet, "/public/kb-articles/slug-id/related", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("slug id: status = %d, want 400", rec.Code)
	}
}

func TestRelatedFiltersMatchesThroughPublishedLookup(t *testing.T) {
	f := &fakeEntity{articles: map[string]entity.KBArticle{
		uuidA: article(uuidA, "published"),
		uuidB: article(uuidB, "published"),
		uuidC: article(uuidC, "draft"),
		uuidD: article(uuidD, "published"),
	}}
	sim := fakeSimilarity{matches: []pinecone.Match{
		{ID: uuidA, Score: 1},                     // the source itself
		{ID: uuidC, Score: 0.95},                  // unpublished
		{ID: uuidMissing, Score: 0.9},             // unknown upstream (404)
		{ID: "ghost", Score: 0.88},                // slug-style id: not a UUID, never looked up
		{ID: "../cases/1", Score: 0.85},           // invalid id, never looked up
		{ID: uuidB, Score: 0.8},                   // good
		{ID: uuidB, Score: 0.7},                   // duplicate
		{ID: strings.ToUpper(uuidB), Score: 0.75}, // duplicate of uuidB in another case
		{ID: strings.ToUpper(uuidD), Score: 0.6},  // good, upper-case in the index
	}}
	rec := do(newMux(f, sim), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got relatedArticlesDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Articles) != 2 || got.Articles[0].ID != uuidB || got.Articles[1].ID != uuidD {
		t.Fatalf("articles = %+v", got.Articles)
	}
	if got.Articles[0].Title != "Title "+uuidB || got.Articles[0].Score != 0.8 {
		t.Errorf("title must come from entity-service, score from the index: %+v", got.Articles[0])
	}
	for _, id := range f.getIDs {
		if n, ok := normalizeID(id); !ok || n != id {
			t.Errorf("non-canonical id %q reached upstream", id)
		}
	}
	if len(f.searchCalls) != 0 {
		t.Error("fallback used although semantic matches survived")
	}
}

func TestRelatedFallsBackToSameKnowledgeBase(t *testing.T) {
	search := entity.SearchResponse{Articles: []entity.KBArticle{
		article(uuidA, "published"), // source, excluded
		article(uuidB, "published"),
		article(uuidC, "draft"), // defensive: must not appear
	}}
	cases := []struct {
		name string
		sim  Similarity
	}{
		{"index disabled", nil},
		{"index error", fakeSimilarity{err: errors.New("down")}},
		{"index returns only unpublished", fakeSimilarity{matches: []pinecone.Match{{ID: uuidC, Score: 1}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeEntity{
				articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published"), uuidC: article(uuidC, "draft")},
				search:   search,
			}
			rec := do(newMux(f, tc.sim), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
			if rec.Code != 200 {
				t.Fatalf("status %d", rec.Code)
			}
			var got relatedArticlesDTO
			_ = json.Unmarshal(rec.Body.Bytes(), &got)
			if len(got.Articles) != 1 || got.Articles[0].ID != uuidB {
				t.Fatalf("articles = %+v", got.Articles)
			}
			if len(f.searchCalls) != 1 || f.searchCalls[0].States[0] != "published" || f.searchCalls[0].KnowledgeBaseID != kbOne {
				t.Errorf("fallback search = %+v", f.searchCalls)
			}
		})
	}
}

func TestRelatedFallbackFailureIsEmptyList(t *testing.T) {
	f := &fakeEntity{
		articles:  map[string]entity.KBArticle{uuidA: article(uuidA, "published")},
		searchErr: errors.New("down"),
	}
	rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"articles":[]}` {
		t.Errorf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestNotFoundAndHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	NotFound(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), `"message"`) {
		t.Errorf("NotFound: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	Health(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != 200 || rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("Health: %d %s", rec.Code, rec.Body)
	}
}

func ptr[T any](v T) *T { return &v }

// inactiveKBEntity has kbOne active and kbOff inactive, with one published
// article in each, so every route can be checked against the inactive one.
func inactiveKBEntity() *fakeEntity {
	off := article(uuidB, "published")
	off.KnowledgeBaseID = kbOff
	return &fakeEntity{
		kbs: []entity.KnowledgeBase{
			{ID: kbOne, Title: "One", Active: true},
			{ID: kbOff, Title: "Off", Active: false},
		},
		articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published"), uuidB: off},
	}
}

func TestInactiveKnowledgeBaseIsNotServed(t *testing.T) {
	t.Run("get article in inactive kb is 404", func(t *testing.T) {
		rec := do(newMux(inactiveKBEntity(), nil), http.MethodGet, "/public/kb-articles/"+uuidB, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
	t.Run("get article in active kb is 200", func(t *testing.T) {
		rec := do(newMux(inactiveKBEntity(), nil), http.MethodGet, "/public/kb-articles/"+uuidA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})
	t.Run("search with inactive knowledgeBaseId is 404 without calling search", func(t *testing.T) {
		f := inactiveKBEntity()
		rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{"knowledgeBaseId":"`+kbOff+`"}`)
		if rec.Code != http.StatusNotFound || len(f.searchCalls) != 0 {
			t.Fatalf("status = %d, searchCalls = %d", rec.Code, len(f.searchCalls))
		}
	})
	t.Run("search with unknown knowledgeBaseId is 404", func(t *testing.T) {
		rec := do(newMux(inactiveKBEntity(), nil), http.MethodPost, "/public/kb-articles/search", `{"knowledgeBaseId":"`+kbUnknown+`"}`)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
	t.Run("search without knowledgeBaseId drops inactive-kb articles", func(t *testing.T) {
		f := inactiveKBEntity()
		off := f.articles[uuidB]
		f.search = entity.SearchResponse{Articles: []entity.KBArticle{article(uuidA, "published"), off}, Total: 2, Limit: 20}
		rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", `{}`)
		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		var got struct {
			Articles []map[string]any `json:"articles"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if len(got.Articles) != 1 || got.Articles[0]["id"] != uuidA {
			t.Errorf("articles = %s", rec.Body)
		}
	})
	t.Run("related for a source in an inactive kb is 404", func(t *testing.T) {
		f := inactiveKBEntity()
		rec := do(newMux(f, fakeSimilarity{matches: []pinecone.Match{{ID: uuidA, Score: 1}}}), http.MethodGet, "/public/kb-articles/"+uuidB+"/related", "")
		if rec.Code != http.StatusNotFound || len(f.searchCalls) != 0 {
			t.Fatalf("status = %d, searchCalls = %d", rec.Code, len(f.searchCalls))
		}
	})
	t.Run("related drops matches in an inactive kb", func(t *testing.T) {
		f := inactiveKBEntity()
		third := article(uuidC, "published")
		f.articles[uuidC] = third
		sim := fakeSimilarity{matches: []pinecone.Match{{ID: uuidB, Score: 0.9}, {ID: uuidC, Score: 0.8}}}
		rec := do(newMux(f, sim), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
		var got relatedArticlesDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != 200 || len(got.Articles) != 1 || got.Articles[0].ID != uuidC {
			t.Fatalf("status %d articles %s", rec.Code, rec.Body)
		}
	})
	t.Run("related fallback drops inactive-kb articles", func(t *testing.T) {
		f := inactiveKBEntity()
		off := f.articles[uuidB]
		same := article(uuidC, "published")
		f.search = entity.SearchResponse{Articles: []entity.KBArticle{off, same}}
		rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA+"/related", "")
		var got relatedArticlesDTO
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if len(got.Articles) != 1 || got.Articles[0].ID != uuidC {
			t.Fatalf("articles %s", rec.Body)
		}
	})
}

func TestActiveKBSetFailsClosed(t *testing.T) {
	routes := []struct{ name, method, target, body string }{
		{"list", http.MethodGet, "/public/knowledge-bases", ""},
		{"search", http.MethodPost, "/public/kb-articles/search", `{}`},
		{"get", http.MethodGet, "/public/kb-articles/" + uuidA, ""},
		{"related", http.MethodGet, "/public/kb-articles/" + uuidA + "/related", ""},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			f := &fakeEntity{
				kbsErr:   &apierror.Error{StatusCode: 500, Body: "pg down"},
				articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published")},
			}
			rec := do(newMux(f, nil), rt.method, rt.target, rt.body)
			if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "pg down") {
				t.Fatalf("status = %d body %s, want generic 502", rec.Code, rec.Body)
			}
		})
	}
	t.Run("draft is 404 even if the set cannot load", func(t *testing.T) {
		f := &fakeEntity{kbsErr: errors.New("down"), articles: map[string]entity.KBArticle{uuidA: article(uuidA, "draft")}}
		rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+uuidA, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestActiveKBCacheTTLAndFailureNotCached(t *testing.T) {
	now := time.Unix(1000, 0)
	f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published")}}
	h := NewKBHandler(f, nil)
	h.activeKBs.now = func() time.Time { return now }
	get := func() int {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /public/kb-articles/{id}", h.GetArticle)
		return do(mux, http.MethodGet, "/public/kb-articles/"+uuidA, "").Code
	}
	if get() != 200 || get() != 200 {
		t.Fatal("want 200")
	}
	if f.kbCalls != 1 {
		t.Fatalf("kb list calls within TTL = %d, want 1", f.kbCalls)
	}
	now = now.Add(activeKBTTL + time.Second)
	// kbOne is deactivated upstream: after the TTL the article stops being served.
	f.mu.Lock()
	f.kbs = []entity.KnowledgeBase{{ID: kbOne, Title: "One", Active: false}}
	f.mu.Unlock()
	if got := get(); got != http.StatusNotFound {
		t.Fatalf("after TTL status = %d, want 404", got)
	}
	if f.kbCalls != 2 {
		t.Fatalf("kb list calls = %d, want 2", f.kbCalls)
	}
	// A failed refresh is not cached and not served stale.
	now = now.Add(activeKBTTL + time.Second)
	f.mu.Lock()
	f.kbsErr = errors.New("down")
	f.mu.Unlock()
	if got := get(); got != http.StatusBadGateway {
		t.Fatalf("failed refresh status = %d, want 502", got)
	}
	f.mu.Lock()
	f.kbsErr = nil
	f.kbs = []entity.KnowledgeBase{{ID: kbOne, Title: "One", Active: true}}
	f.mu.Unlock()
	if got := get(); got != 200 {
		t.Fatalf("recovery status = %d, want 200", got)
	}
}

// TestActiveKBCacheConcurrent: N callers arriving together inside the TTL must
// cause exactly one upstream list call. The load is held open until every caller
// has started, so they genuinely overlap the in-flight load.
func TestActiveKBCacheConcurrent(t *testing.T) {
	const n = 50
	gate := make(chan struct{})
	f := &fakeEntity{
		articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published")},
		kbGate:   gate,
	}
	mux := newMux(f, nil)
	var started, wg sync.WaitGroup
	started.Add(n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started.Done()
			if rec := do(mux, http.MethodGet, "/public/kb-articles/"+uuidA, ""); rec.Code != 200 {
				t.Errorf("status %d", rec.Code)
			}
		}()
	}
	started.Wait()
	time.Sleep(50 * time.Millisecond) // let the callers reach the cache
	close(gate)
	wg.Wait()
	if f.kbCalls != 1 {
		t.Errorf("kb list calls for %d concurrent callers = %d, want exactly 1", n, f.kbCalls)
	}
	// And a later caller inside the TTL still does not reload.
	if rec := do(mux, http.MethodGet, "/public/kb-articles/"+uuidA, ""); rec.Code != 200 || f.kbCalls != 1 {
		t.Errorf("follow-up: status %d, kb list calls %d, want 200 and 1", rec.Code, f.kbCalls)
	}
}

// TestActiveKBCacheLoaderPanic: a panicking loader must not leave a dangling
// in-flight marker, or every later caller would block forever.
func TestActiveKBCacheLoaderPanic(t *testing.T) {
	var calls int
	c := newActiveKBCache(func(context.Context) ([]entity.KnowledgeBase, error) {
		calls++
		if calls == 1 {
			panic("loader exploded")
		}
		return []entity.KnowledgeBase{{ID: kbOne, Title: "One", Active: true}}, nil
	})

	func() {
		defer func() {
			if recover() == nil {
				t.Error("the loader's panic should propagate to the caller that ran it")
			}
		}()
		_, _ = c.get(context.Background())
	}()

	c.mu.Lock()
	inflight := c.inflight
	c.mu.Unlock()
	if inflight != nil {
		t.Fatal("in-flight marker left behind after a loader panic")
	}

	type result struct {
		set kbSet
		err error
	}
	ch := make(chan result, 1)
	go func() {
		set, err := c.get(context.Background())
		ch <- result{set, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil || !r.set.has(kbOne) {
			t.Fatalf("next call after a panic: set %v err %v, want success", r.set, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("next call after a loader panic blocked")
	}
}

// TestActiveKBCacheWaiterSeesLoaderPanic: a caller already waiting on a flight
// whose loader panics is released with an error, not left hanging.
func TestActiveKBCacheWaiterSeesLoaderPanic(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	c := newActiveKBCache(func(context.Context) ([]entity.KnowledgeBase, error) {
		close(entered)
		<-release
		panic("loader exploded")
	})
	go func() {
		defer func() { _ = recover() }()
		_, _ = c.get(context.Background())
	}()
	<-entered
	errc := make(chan error, 1)
	go func() {
		_, err := c.get(context.Background())
		errc <- err
	}()
	time.Sleep(50 * time.Millisecond) // let the waiter attach to the flight
	close(release)
	select {
	case err := <-errc:
		if !errors.Is(err, errLoadPanicked) {
			t.Fatalf("waiter err = %v, want errLoadPanicked", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter blocked after the loader panicked")
	}
}

func TestSearchKnowledgeBaseIDValidatedBeforeUpstream(t *testing.T) {
	for _, id := range []string{"../x", "a/b", "a%2Fb", "a b", strings.Repeat("a", 65), "kb-1"} {
		f := &fakeEntity{}
		body, _ := json.Marshal(map[string]string{"knowledgeBaseId": id})
		rec := do(newMux(f, nil), http.MethodPost, "/public/kb-articles/search", string(body))
		if rec.Code != http.StatusBadRequest || len(f.searchCalls) != 0 || f.kbCalls != 0 {
			t.Errorf("%q: status %d searchCalls %d kbCalls %d", id, rec.Code, len(f.searchCalls), f.kbCalls)
		}
	}
}

func TestGetArticleTraversalIDIs400(t *testing.T) {
	f := &fakeEntity{}
	rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/%2F..%2Fcases%2F1", "")
	if rec.Code != http.StatusBadRequest || len(f.getIDs) != 0 {
		t.Fatalf("status %d getIDs %v", rec.Code, f.getIDs)
	}
}

// TestNonexistentUUIDIs404OnEveryRoute: the fake answers like the real upstream
// (a missing article is a 404, a malformed id a 400), and the portal turns the
// lookup of a well-formed but nonexistent id into its own generic 404.
func TestNonexistentUUIDIs404OnEveryRoute(t *testing.T) {
	routes := []struct{ name, method, target, body string }{
		{"get", http.MethodGet, "/public/kb-articles/" + uuidMissing, ""},
		{"get upper case", http.MethodGet, "/public/kb-articles/" + strings.ToUpper(uuidMissing), ""},
		{"related", http.MethodGet, "/public/kb-articles/" + uuidMissing + "/related", ""},
		{"search with unknown knowledgeBaseId", http.MethodPost, "/public/kb-articles/search", `{"knowledgeBaseId":"` + uuidMissing + `"}`},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published")}}
			rec := do(newMux(f, nil), rt.method, rt.target, rt.body)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 (body %s)", rec.Code, rec.Body)
			}
			var e errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Message != ErrMsgNotFound {
				t.Errorf("body = %s, want the generic not-found envelope", rec.Body)
			}
			if strings.Contains(rec.Body.String(), "article not found") {
				t.Errorf("upstream body relayed: %s", rec.Body)
			}
		})
	}
}

// TestFakeMirrorsUpstreamStatuses pins the fake to the real entity-service
// contract so a test can never pass on behaviour the real service lacks.
func TestFakeMirrorsUpstreamStatuses(t *testing.T) {
	f := &fakeEntity{}
	var apiErr *apierror.Error
	if _, err := f.GetKBArticle(context.Background(), uuidMissing); !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("missing id: err = %v, want upstream 404", err)
	}
	if _, err := f.GetKBArticle(context.Background(), "not-a-uuid"); !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed id: err = %v, want upstream 400", err)
	}
}

func TestUpperCaseIDIsNormalisedBeforeUpstream(t *testing.T) {
	f := &fakeEntity{articles: map[string]entity.KBArticle{uuidA: article(uuidA, "published")}}
	rec := do(newMux(f, nil), http.MethodGet, "/public/kb-articles/"+strings.ToUpper(uuidA), "")
	if rec.Code != 200 || len(f.getIDs) != 1 || f.getIDs[0] != uuidA {
		t.Fatalf("status %d, upstream ids %v, want 200 with the lower-case id", rec.Code, f.getIDs)
	}
}
