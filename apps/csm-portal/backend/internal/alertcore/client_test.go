// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package alertcore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

func TestMutedIncidentsSendsTokenAndPage(t *testing.T) {
	var gotAuth, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotQuery = r.Header.Get("Authorization"), r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"total":1}`))
	}))
	defer srv.Close()
	body, err := NewClient(Config{BaseURL: srv.URL + "/", Token: "secret"}).MutedIncidents(context.Background(), "abc", 20, 40)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"total":1}` || gotAuth != "Bearer secret" || gotPath != "/alert-mute-rules/abc/muted-incidents" || gotQuery != "limit=20&offset=40" {
		t.Fatalf("body=%s auth=%q path=%q query=%q", body, gotAuth, gotPath, gotQuery)
	}
}

func TestMutedIncidentsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()
	_, err := NewClient(Config{BaseURL: srv.URL, Token: "t"}).MutedIncidents(context.Background(), "abc", 20, 0)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want an apierror with 401", err)
	}
}

func TestFieldValuesSendsFieldAndQuery(t *testing.T) {
	var gotAuth, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotQuery = r.Header.Get("Authorization"), r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"values":[]}`))
	}))
	defer srv.Close()
	body, err := NewClient(Config{BaseURL: srv.URL, Token: "secret"}).FieldValues(context.Background(), "service", "client prod", 10)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"values":[]}` || gotAuth != "Bearer secret" || gotPath != "/alert-field-values" ||
		gotQuery != "field=service&limit=10&q=client+prod" {
		t.Fatalf("body=%s auth=%q path=%q query=%q", body, gotAuth, gotPath, gotQuery)
	}
}

func TestGetRejectsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, maxBody+1))
	}))
	defer srv.Close()
	if _, err := NewClient(Config{BaseURL: srv.URL, Token: "t"}).MutedIncidents(context.Background(), "abc", 20, 0); err == nil {
		t.Fatal("want an error for a body over maxBody, got nil")
	}
}

func TestMutedCountsSendsIDs(t *testing.T) {
	var gotPath, gotIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotIDs = r.URL.Path, r.URL.Query().Get("ids")
		_, _ = w.Write([]byte(`{"counts":{}}`))
	}))
	defer srv.Close()
	if _, err := NewClient(Config{BaseURL: srv.URL, Token: "t"}).MutedCounts(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/alert-mute-rules/muted-counts" || gotIDs != "a,b" {
		t.Fatalf("path=%q ids=%q", gotPath, gotIDs)
	}
}
