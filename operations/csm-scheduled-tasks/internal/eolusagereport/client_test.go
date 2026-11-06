package eolusagereport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

func TestReportSendsAsOfAndDecodes(t *testing.T) {
	var gotPath, gotAsOf string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAsOf = r.URL.Path, r.URL.Query().Get("asOf")
		_, _ = w.Write([]byte(`{"generatedOn":"2026-11-02T04:30:00Z","asOf":"2026-11-02","totalVersions":1,"totalAccounts":1,
			"regions":[{"region":"APAC","accounts":[{"accountId":"a1","accountName":"Acme","csAbt":"Team Alpha","region":"APAC",
			"accountManager":{"name":"Account Manager One","email":"am.one@example.com"},"technicalOwner":null,
			"projects":[{"projectId":"p1","projectName":"Acme Sub","projectKey":"ACMESUB",
			"versions":[{"productVersionId":"v1","productName":"WSO2 API Manager","version":"2.6.0","supportStatus":"EXTENDED","eolDate":"2022-10-22"}]}]}]},
			{"region":null,"accounts":[{"accountId":"a2","accountName":"Beta","csAbt":null,"region":null,
			"accountManager":{"email":"am.two@example.com"},"technicalOwner":null,
			"projects":[{"projectId":"p2","projectName":"Beta Prod","projectKey":null,
			"versions":[{"productVersionId":"v2","productName":"WSO2 Identity Server","version":"5.10.0","supportStatus":null,"eolDate":"2026-03-25"}]}]}]}]}`))
	}))
	defer srv.Close()

	c := &Client{http: srv.Client(), baseURL: srv.URL}
	r, err := c.Report(context.Background(), "2026-11-02")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/reports/eol-product-usage" || gotAsOf != "2026-11-02" {
		t.Errorf("request = %s?asOf=%s", gotPath, gotAsOf)
	}
	acc := r.Regions[0].Accounts[0]
	v := acc.Projects[0].Versions[0]
	if r.TotalVersions != 1 || acc.AccountManager == nil || acc.AccountManager.Email != "am.one@example.com" ||
		acc.TechnicalOwner != nil || v.SupportStatus == nil || *v.SupportStatus != "EXTENDED" || v.EOLDate != "2022-10-22" ||
		acc.Projects[0].ProjectKey != "ACMESUB" || !r.GeneratedOn.Equal(time.Date(2026, 11, 2, 4, 30, 0, 0, time.UTC)) {
		t.Errorf("decoded = %+v", r)
	}
	// Nulls and a missing name decode to "".
	none := r.Regions[1].Accounts[0]
	if r.Regions[1].Region != "" || none.CsAbt != "" || none.Projects[0].ProjectKey != "" ||
		none.AccountManager == nil || none.AccountManager.Name != "" || none.AccountManager.Email != "am.two@example.com" {
		t.Errorf("no-region account = %+v", none)
	}
}

func TestReportReturnsUpstreamErrors(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"down"}`, http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := &Client{http: srv.Client(), baseURL: srv.URL}
	_, err := c.Report(context.Background(), "2026-11-02")
	apiErr, ok := err.(*apierror.Error)
	if !ok || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want an *apierror.Error with 503", err)
	}
}

func TestNewClientRequiresHTTPS(t *testing.T) {
	if _, err := NewClient(Config{BaseURL: "http://entity", TokenURL: "https://idp/token"}); err == nil {
		t.Error("an http base URL must be refused")
	}
}
