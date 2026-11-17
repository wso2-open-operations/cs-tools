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
package moesif

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	from = time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	to   = time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
)

func hit(id string, sortKey int64) string {
	return fmt.Sprintf(`{"_id":%q,"sort":[%d],"_source":{"company_id":"c-%s","request":{"time":"2026-11-08T01:00:00.000"},"metadata":{"product":"core"},"ip":"10.0.0.1"}}`, id, sortKey, id)
}

// serve answers each search with the next page and records the queries.
func serve(t *testing.T, status int, pages []string, queries *[]map[string]any) *Client {
	t.Helper()
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search/~/search/events" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("from"); got != "2026-11-08T00:00:00.000Z" {
			t.Errorf("from = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("authorization = %q", got)
		}
		if queries != nil {
			raw, _ := io.ReadAll(r.Body)
			var q map[string]any
			_ = json.Unmarshal(raw, &q)
			*queries = append(*queries, q)
		}
		w.WriteHeader(status)
		page := ""
		if call < len(pages) {
			page = pages[call]
		}
		call++
		_, _ = io.WriteString(w, `{"hits":{"hits":[`+page+`]}}`)
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{BaseURL: srv.URL + "/v1", APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func query() Query {
	return Query{From: from, To: to, Filter: TermsFilter("action_name.raw", []string{"a", "b"}),
		Fields: []string{"request.time", "company_id"}, PageSize: 2, MaxPages: 2}
}

func TestSearchEventsPagesWithTheLastSortKey(t *testing.T) {
	var queries []map[string]any
	c := serve(t, 200, []string{hit("1", 1794103200000) + "," + hit("2", 1794103200001), hit("3", 1794103200002)}, &queries)

	hits, more, err := c.SearchEvents(context.Background(), query())
	if err != nil || more {
		t.Fatalf("err = %v, more = %v", err, more)
	}
	if len(hits) != 3 || hits[0].ID != "1" || Text(hits[2].Source, "metadata.product") != "core" {
		t.Fatalf("hits = %+v", hits)
	}
	if len(queries) != 2 {
		t.Fatalf("queries = %d, want 2", len(queries))
	}
	if queries[0]["search_after"] != nil {
		t.Errorf("first page sent search_after")
	}
	if got := queries[1]["search_after"]; !reflect.DeepEqual(got, []any{float64(1794103200001)}) {
		t.Errorf("search_after = %v, want the last sort key, exact", got)
	}
	if got := queries[0]["_source"]; !reflect.DeepEqual(got, []any{"request.time", "company_id"}) {
		t.Errorf("_source = %v, want only the fields asked for", got)
	}
	want := map[string]any{"bool": map[string]any{"must": []any{map[string]any{"terms": map[string]any{"action_name.raw": []any{"a", "b"}}}}}}
	if !reflect.DeepEqual(queries[0]["post_filter"], want) {
		t.Errorf("post_filter = %v", queries[0]["post_filter"])
	}
}

func TestSearchEventsReportsMoreAtThePageLimit(t *testing.T) {
	full := hit("1", 1) + "," + hit("2", 2)
	hits, more, err := serve(t, 200, []string{full, full}, nil).SearchEvents(context.Background(), query())
	if err != nil || !more || len(hits) != 4 {
		t.Fatalf("hits = %d, more = %v, err = %v", len(hits), more, err)
	}
}

func TestARateLimitOrServerErrorIsUnavailableAndBodiesAreNeverReturned(t *testing.T) {
	for _, status := range []int{429, 503} {
		_, _, err := serve(t, status, []string{hit("x", 1)}, nil).SearchEvents(context.Background(), query())
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("status %d: err = %v, want ErrUnavailable", status, err)
		}
	}
	_, _, err := serve(t, 401, []string{hit("secret-body", 1)}, nil).SearchEvents(context.Background(), query())
	if err == nil || errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "secret-body") {
		t.Errorf("401: err = %v, want a plain refusal without the body", err)
	}
}

func TestNewClientNeedsAKeyAndHTTPS(t *testing.T) {
	if _, err := NewClient(Config{APIKey: ""}); err == nil {
		t.Error("an empty key was accepted")
	}
	if _, err := NewClient(Config{BaseURL: "http://moesif.example", APIKey: "k"}); err == nil {
		t.Error("a plain-http host was accepted")
	}
}

func TestTextAndParseTime(t *testing.T) {
	if Text(map[string]any{"metadata.product": "core"}, "metadata.product") != "core" ||
		Text(map[string]any{"metadata": map[string]any{"product": " core "}}, "metadata.product") != "core" ||
		Text(map[string]any{"metadata": nil}, "metadata.product") != "" {
		t.Error("Text did not read a literal or nested path")
	}
	for _, s := range []string{"2026-11-08T15:27:33.928", "2026-11-08T15:27:33.928Z"} {
		if got, ok := ParseTime(s); !ok || !got.Equal(time.Date(2026, 11, 8, 15, 27, 33, 928e6, time.UTC)) {
			t.Errorf("ParseTime(%q) = %v, %v", s, got, ok)
		}
	}
}
