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

// Tests BuildTitlesQuery's repo/alias grouping and FetchTitles' batched
// title resolution, including per-issue nulls vs whole-batch failures.
package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestBuildTitlesQueryGroupsByRepoWithAliasSafeNamesAndEscapedStrings
// verifies refs across repos are grouped under distinct r<i>/n<number>
// aliases with owner/name escaped as GraphQL string literals.
func TestBuildTitlesQueryGroupsByRepoWithAliasSafeNamesAndEscapedStrings(t *testing.T) {
	q := BuildTitlesQuery([]TitleRef{
		{ID: 1, Owner: "wso2-enterprise", Name: "wso2-iam-internal", Number: 7366},
		{ID: 2, Owner: "wso2-enterprise", Name: "wso2-iam-internal", Number: 12},
		{ID: 3, Owner: "wso2-enterprise", Name: "wso2-apim-internal", Number: 99},
	})

	for _, want := range []string{
		`r0: repository(owner: "wso2-enterprise", name: "wso2-iam-internal")`,
		"n7366: issue(number: 7366) { title }",
		`r1: repository(owner: "wso2-enterprise", name: "wso2-apim-internal")`,
		"n99: issue(number: 99) { title }",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("expected query to contain %q, got:\n%s", want, q)
		}
	}
}

// TestFetchTitlesReturnsEmptyMapWithNoTokenOrNoRefs verifies FetchTitles
// short-circuits to an empty map, nil error for a blank token or no refs,
// without making a network call.
func TestFetchTitlesReturnsEmptyMapWithNoTokenOrNoRefs(t *testing.T) {
	out, err := FetchTitles(context.Background(), "", []TitleRef{{ID: 1, Owner: "o", Name: "n", Number: 1}})
	if err != nil || len(out) != 0 {
		t.Errorf("expected empty map, nil error for blank token, got %v, %v", out, err)
	}

	out, err = FetchTitles(context.Background(), "token", nil)
	if err != nil || len(out) != 0 {
		t.Errorf("expected empty map, nil error for no refs, got %v, %v", out, err)
	}
}

// newTitlesTestServer points graphQLPath at a local httptest.Server running
// handler for the duration of t, restoring it on cleanup.
func newTitlesTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	orig := graphQLPath
	graphQLPath = srv.URL
	t.Cleanup(func() { graphQLPath = orig })
}

// TestFetchTitlesResolvesPerIssueNullsAndTitles verifies a resolved title
// comes back non-nil while a deleted/missing issue resolves to a nil title,
// without failing the whole batch.
func TestFetchTitlesResolvesPerIssueNullsAndTitles(t *testing.T) {
	newTitlesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"r0":{"n7366":{"title":"Fix the widget"},"n12":null}}}`))
	})

	out, err := FetchTitles(context.Background(), "test-token", []TitleRef{
		{ID: 1, Owner: "acme", Name: "widgets", Number: 7366},
		{ID: 2, Owner: "acme", Name: "widgets", Number: 12},
	})
	if err != nil {
		t.Fatalf("FetchTitles: %v", err)
	}
	if title := out[RefKey(TitleRef{Owner: "acme", Name: "widgets", Number: 7366})]; title == nil || *title != "Fix the widget" {
		t.Errorf("expected resolved title, got %v", title)
	}
	if title := out[RefKey(TitleRef{Owner: "acme", Name: "widgets", Number: 12})]; title != nil {
		t.Errorf("expected nil title for deleted/missing issue, got %v", *title)
	}
}

// TestFetchTitlesWholeBatchFailureOnNullData verifies an HTTP 200 with
// data: null (e.g. missing scope) fails the whole batch with that GraphQL
// error message, rather than caching per-issue nulls.
func TestFetchTitlesWholeBatchFailureOnNullData(t *testing.T) {
	newTitlesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"missing scope"}]}`))
	})

	_, err := FetchTitles(context.Background(), "test-token", []TitleRef{{ID: 1, Owner: "acme", Name: "widgets", Number: 1}})
	if err == nil || !strings.Contains(err.Error(), "missing scope") {
		t.Fatalf("expected a whole-batch error mentioning 'missing scope', got %v", err)
	}
}

// TestFetchTitlesHTTPErrorFails verifies a non-200 response returns an
// error instead of a partial or empty result.
func TestFetchTitlesHTTPErrorFails(t *testing.T) {
	newTitlesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := FetchTitles(context.Background(), "test-token", []TitleRef{{ID: 1, Owner: "acme", Name: "widgets", Number: 1}})
	if err == nil {
		t.Fatal("expected an error on HTTP 500")
	}
}
