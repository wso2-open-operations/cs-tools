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

package productissuereport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

type authTransport struct{ next http.RoundTripper }

func (a authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer test-token")
	return a.next.RoundTrip(r)
}

func testClient(srv *httptest.Server) *Client {
	return &Client{http: &http.Client{Transport: authTransport{http.DefaultTransport}}, baseURL: srv.URL}
}

func TestDashboardQueryAndDecode(t *testing.T) {
	var gotPath, gotAuth string
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotQuery = r.URL.Path, r.Header.Get("Authorization"), r.URL.Query()
		_, _ = w.Write([]byte(`{"periodStart":"2026-11-09T00:00:00+05:30","periodEnd":"2026-11-16T00:00:00+05:30","totals":{"open":3,"compliancePercentage":null},"byProduct":[{"productName":null,"owner":"o","repository":"r","compliancePercentage":87.5}]}`))
	}))
	defer srv.Close()

	d, err := testClient(srv).Dashboard(context.Background(), PeriodWeek, "2026-11-09", []string{"API Manager", "Identity Server"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/reports/product-issues" || gotAuth != "Bearer test-token" {
		t.Errorf("path=%q auth=%q", gotPath, gotAuth)
	}
	if gotQuery["period"][0] != "week" || gotQuery["date"][0] != "2026-11-09" || gotQuery["products"][0] != "API Manager,Identity Server" {
		t.Errorf("query = %v", gotQuery)
	}
	if d.Totals.Open != 3 || d.Totals.CompliancePercentage != nil {
		t.Errorf("totals = %+v", d.Totals)
	}
	if d.ByProduct[0].ProductName != nil || d.ByProduct[0].CompliancePercentage == nil || *d.ByProduct[0].CompliancePercentage != 87.5 {
		t.Errorf("by product = %+v", d.ByProduct[0])
	}
}

func TestDashboardOmitsEmptyOptionalParams(t *testing.T) {
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if _, err := testClient(srv).Dashboard(context.Background(), PeriodMonth, "", nil); err != nil {
		t.Fatal(err)
	}
	if gotRaw != "period=month" {
		t.Errorf("raw query = %q", gotRaw)
	}
}

func TestDashboardEncodesSpecialCharacters(t *testing.T) {
	var gotProducts string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotProducts = r.URL.Query().Get("products")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if _, err := testClient(srv).Dashboard(context.Background(), PeriodWeek, "", []string{"A&B=C", "D E"}); err != nil {
		t.Fatal(err)
	}
	if gotProducts != "A&B=C,D E" {
		t.Errorf("products = %q", gotProducts)
	}
}

func TestRecipients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reports/product-issues/recipients" || r.Header.Get("Authorization") == "" {
			http.Error(w, "bad", 400)
			return
		}
		_, _ = w.Write([]byte(`{"leads":[{"email":"a@x.invalid","productNames":["API Manager"]}]}`))
	}))
	defer srv.Close()
	leads, err := testClient(srv).Recipients(context.Background())
	if err != nil || len(leads) != 1 || leads[0].Email != "a@x.invalid" || leads[0].ProductNames[0] != "API Manager" {
		t.Fatalf("leads=%v err=%v", leads, err)
	}
}

func TestNon200IsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "secret detail", http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := testClient(srv).Recipients(context.Background())
	var ae *apierror.Error
	if !errors.As(err, &ae) || ae.StatusCode != 404 {
		t.Fatalf("err = %v", err)
	}
	if got := err.Error(); got != "upstream returned 404" {
		t.Errorf("message leaks body: %q", got)
	}
}

func TestNewClientRequiresHTTPS(t *testing.T) {
	good := Config{BaseURL: "https://es.example.com", TokenURL: "https://idp.example.com/token"}
	if _, err := NewClient(good); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.BaseURL = "http://es.example.com"
	if _, err := NewClient(bad); err == nil {
		t.Error("http base URL accepted")
	}
	bad = good
	bad.TokenURL = "http://idp.example.com/token"
	if _, err := NewClient(bad); err == nil {
		t.Error("http token URL accepted")
	}
}

func TestContainsComma(t *testing.T) {
	if ContainsComma([]string{"A", "B"}) || !ContainsComma([]string{"A", "B, C"}) {
		t.Error("ContainsComma wrong")
	}
}
