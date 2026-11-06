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

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/apierror"
)

func plainClient(baseURL string) *Client {
	return newClient(&http.Client{}, baseURL, 2*time.Second)
}

func TestGetKBArticleEscapesID(t *testing.T) {
	var gotPath, gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotRaw = r.URL.Path, r.URL.RawPath
		if r.URL.RawQuery != "" {
			t.Errorf("query reached upstream: %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"id":"x","state":"published"}`))
	}))
	defer srv.Close()

	// A hostile id that slipped past validation must stay one path segment.
	if _, err := plainClient(srv.URL).GetKBArticle(context.Background(), "../cases/1?state=draft#x"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotRaw, "/kb-articles/") || strings.Count(gotRaw, "/") != 2 {
		t.Errorf("raw path = %q, want a single escaped segment under /kb-articles/", gotRaw)
	}
	if gotPath != "/kb-articles/../cases/1?state=draft#x" {
		t.Errorf("decoded path = %q", gotPath)
	}
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	var targetHit atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit.Store(true)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/steal", http.StatusFound)
	}))
	defer srv.Close()

	_, err := plainClient(srv.URL).GetKBArticle(context.Background(), "abc")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound {
		t.Fatalf("err = %v, want apierror with 302", err)
	}
	if targetHit.Load() {
		t.Error("client followed the redirect")
	}
}

func TestSearchSendsPublishedStateAndNoSortBy(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/kb-articles/search" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-CSM-Correlation-ID") != "corr-1" {
			t.Errorf("correlation id not forwarded: %q", r.Header.Get("X-CSM-Correlation-ID"))
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"articles":[],"total":0,"limit":20,"offset":0,"hasMore":false}`))
	}))
	defer srv.Close()

	ctx := WithCorrelationID(context.Background(), "corr-1")
	_, err := plainClient(srv.URL).SearchKBArticles(ctx, SearchRequest{
		KnowledgeBaseID: "kb-1", States: []string{StatePublished}, SearchQuery: "q",
		Pagination: Pagination{Limit: 5, Offset: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	// entity-service decodes with DisallowUnknownFields: only its own keys may be sent.
	allowed := map[string]bool{"knowledgeBaseId": true, "states": true, "authorId": true, "teamKeys": true, "searchQuery": true, "pagination": true}
	for k := range body {
		if !allowed[k] {
			t.Errorf("key %q is not in entity-service's SearchKBArticlesRequest", k)
		}
	}
	if st, _ := body["states"].([]any); len(st) != 1 || st[0] != "published" {
		t.Errorf("states = %v", body["states"])
	}
}

func TestErrorStatusBecomesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(strings.Repeat("x", 5000)))
	}))
	defer srv.Close()
	_, err := plainClient(srv.URL).GetKBArticle(context.Background(), "abc")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Fatalf("err = %v", err)
	}
	if len(apiErr.Body) > maxErrBody {
		t.Errorf("error body kept %d bytes, cap %d", len(apiErr.Body), maxErrBody)
	}
}

func TestOversizedAndMalformedResponses(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"title":"` + strings.Repeat("a", maxResponseBytes) + `"}`))
	}))
	defer big.Close()
	if _, err := plainClient(big.URL).GetKBArticle(context.Background(), "abc"); err == nil {
		t.Error("oversized response accepted")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer bad.Close()
	if _, err := plainClient(bad.URL).GetKBArticle(context.Background(), "abc"); err == nil {
		t.Error("malformed response accepted")
	}
}

func TestClientCredentialsTokenIsSentAndTrailingSlashTrimmed(t *testing.T) {
	var gotAuth, gotPath string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		_, _ = w.Write([]byte(`{"knowledgeBases":[{"id":"kb-1","title":"One","active":true}]}`))
	}))
	defer api.Close()
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"svc-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer idp.Close()

	c := NewClient(Config{BaseURL: api.URL + "/", TokenURL: idp.URL, ClientID: "id", ClientSecret: "s"})
	kbs, err := c.ListKnowledgeBases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer svc-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotPath != "/knowledge-bases" {
		t.Errorf("path = %q", gotPath)
	}
	if len(kbs) != 1 || kbs[0].ID != "kb-1" || !kbs[0].Active {
		t.Errorf("kbs = %+v", kbs)
	}
}

// The payloads below are byte-for-byte the shapes entity-service writes:
// GetKBArticle encodes the domain.KBArticle itself (flat, no wrapper),
// SearchKBArticles encodes SearchKBArticlesResponse and ListKnowledgeBases
// encodes ListKnowledgeBasesResponse.
const (
	realArticleJSON = `{"id":"11111111-1111-1111-1111-111111111111","knowledgeBaseId":"kb-1","title":"T","body":"<p>b</p>",
		"state":"published","authorId":"a","revisedById":null,"sourceCaseId":null,"rejectionComment":null,"updatedBy":null,
		"baseVersionId":null,"latest":true,"createdOn":"2026-01-01T00:00:00Z","updatedOn":"2026-01-02T00:00:00Z",
		"publishedOn":"2026-01-03T00:00:00Z","retiredOn":null}`
	realSearchJSON = `{"articles":[` + realArticleJSON + `],"total":1,"limit":20,"offset":0,"hasMore":false}`
	realKBsJSON    = `{"knowledgeBases":[{"id":"kb-1","title":"One","active":true,"createdOn":"2026-01-01T00:00:00Z","updatedOn":"2026-01-01T00:00:00Z"}]}`
)

func stub(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestDecodesRealEntityServiceShapes(t *testing.T) {
	t.Run("get article is flat", func(t *testing.T) {
		srv := stub(realArticleJSON)
		defer srv.Close()
		a, err := plainClient(srv.URL).GetKBArticle(context.Background(), "11111111-1111-1111-1111-111111111111")
		if err != nil || a.ID == "" || a.Title != "T" || a.State != StatePublished || a.KnowledgeBaseID != "kb-1" || a.PublishedOn == nil {
			t.Fatalf("a = %+v err = %v", a, err)
		}
	})
	t.Run("a wrapped article is not accepted as an article", func(t *testing.T) {
		// create/state responses wrap in {"article": ...}; get does not. Decoding the
		// wrapped form must not yield a published article by accident.
		srv := stub(`{"article":` + realArticleJSON + `}`)
		defer srv.Close()
		a, _ := plainClient(srv.URL).GetKBArticle(context.Background(), "abc")
		if a.State == StatePublished || a.ID != "" {
			t.Fatalf("wrapped payload decoded as an article: %+v", a)
		}
	})
	t.Run("search", func(t *testing.T) {
		srv := stub(realSearchJSON)
		defer srv.Close()
		r, err := plainClient(srv.URL).SearchKBArticles(context.Background(), SearchRequest{States: []string{StatePublished}})
		if err != nil || len(r.Articles) != 1 || r.Total != 1 || r.Limit != 20 || r.HasMore || r.Articles[0].Title != "T" {
			t.Fatalf("r = %+v err = %v", r, err)
		}
	})
	t.Run("knowledge bases", func(t *testing.T) {
		srv := stub(realKBsJSON)
		defer srv.Close()
		kbs, err := plainClient(srv.URL).ListKnowledgeBases(context.Background())
		if err != nil || len(kbs) != 1 || kbs[0].ID != "kb-1" || kbs[0].Title != "One" || !kbs[0].Active {
			t.Fatalf("kbs = %+v err = %v", kbs, err)
		}
	})
}
