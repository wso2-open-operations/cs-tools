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
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/productconsumption"
)

const (
	testLicenseProjectID    = "11111111-1111-1111-1111-111111111111"
	testLicenseDeploymentID = "22222222-2222-2222-2222-222222222222"
)

// fakeLicenseProductConsumptionClient records the LicenseDownloadRequest it
// was called with and returns a fixed license (or error).
type fakeLicenseProductConsumptionClient struct {
	gotReq  productconsumption.LicenseDownloadRequest
	calls   int
	license productconsumption.License
	err     error

	gotImportEmail string
	gotImportZip   []byte
	importCalls    int
	importRes      productconsumption.ImportUsageResponse
	importErr      error
}

func (f *fakeLicenseProductConsumptionClient) ProcessLicenseDownload(_ context.Context, req productconsumption.LicenseDownloadRequest) (productconsumption.License, error) {
	f.gotReq = req
	f.calls++
	return f.license, f.err
}

func (f *fakeLicenseProductConsumptionClient) ImportDeploymentUsage(_ context.Context, email string, zipFile []byte) (productconsumption.ImportUsageResponse, error) {
	f.gotImportEmail = email
	f.gotImportZip = zipFile
	f.importCalls++
	return f.importRes, f.importErr
}

// fakeProjectAccessChecker controls whether GetProject succeeds, standing in
// for entity-service's own project-access gate.
type fakeProjectAccessChecker struct {
	err   error
	calls int

	// SearchDeployments: the deployments the caller can see, by number.
	visibleDeployments map[string]bool
	searchErr          error
	gotSearch          entity.SearchDeploymentsRequest
	searchCalls        int
}

func (f *fakeProjectAccessChecker) GetProject(context.Context, string) (entity.ProjectDetailsView, error) {
	f.calls++
	return entity.ProjectDetailsView{}, f.err
}

func (f *fakeProjectAccessChecker) SearchDeployments(_ context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error) {
	f.searchCalls++
	f.gotSearch = req
	if f.searchErr != nil {
		return entity.SearchDeploymentsResponse{}, f.searchErr
	}
	var resp entity.SearchDeploymentsResponse
	for _, n := range req.Numbers {
		if f.visibleDeployments[n] {
			resp.Deployments = append(resp.Deployments, entity.DeploymentView{})
		}
	}
	return resp, nil
}

// usageReportZip builds a ZIP the way WSO2 products export a usage report: one
// JSON file whose export_metadata names the deployment.
func usageReportZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const testUsageReport = `{"export_metadata":{"export_version":"1.0","is_valid":true,"deployment_id":"DEP000002442","signature":"sig"},` +
	`"deployment_meta_information":[],"deployment_information":[],"daily_usage_count":[]}`

func newLicenseMux(product productConsumptionClient, entityClient entityProjectAccessChecker) *http.ServeMux {
	h := NewProductConsumptionHandler(product, entityClient)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /projects/{projectId}/deployments/{deploymentId}/license", h.GetDeploymentLicense)
	return mux
}

// TestGetDeploymentLicense_RejectsWhenProjectAccessFails confirms the
// project-access gate runs before ProcessLicenseDownload is ever called —
// entity-service's GetProject failing must not still provision anything.
func TestGetDeploymentLicense_RejectsWhenProjectAccessFails(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{}
	entityClient := &fakeProjectAccessChecker{err: apierror.NewUpstreamError(http.StatusNotFound, []byte(`{"message":"project not found"}`))}

	mux := newLicenseMux(product, entityClient)
	req := authedRequest(http.MethodPost, "/projects/"+testLicenseProjectID+"/deployments/"+testLicenseDeploymentID+"/license", "")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if product.calls != 0 {
		t.Fatalf("ProcessLicenseDownload calls = %d, want 0 — project access failed, nothing should have been provisioned", product.calls)
	}
}

// TestGetDeploymentLicense_ForwardsRequestAndReturnsLicense is the success
// path: the caller's email and the path's project/deployment ids reach
// ProcessLicenseDownload unchanged, and the signed subscription data comes
// back through dto.MapLicense byte for byte.
func TestGetDeploymentLicense_ForwardsRequestAndReturnsLicense(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{
		license: productconsumption.License{
			SubscriptionData: json.RawMessage(`{"clientId":"abc","usageDataPublishingUrl":"https://example.com"}`),
			Signature:        "sig-value",
		},
	}
	entityClient := &fakeProjectAccessChecker{}

	mux := newLicenseMux(product, entityClient)
	req := authedRequest(http.MethodPost, "/projects/"+testLicenseProjectID+"/deployments/"+testLicenseDeploymentID+"/license", "")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if entityClient.calls != 1 {
		t.Fatalf("GetProject calls = %d, want 1", entityClient.calls)
	}
	if product.gotReq.Email != "customer@example.com" {
		t.Errorf("Email = %q, want the authenticated caller's email", product.gotReq.Email)
	}
	if product.gotReq.ProjectID != testLicenseProjectID {
		t.Errorf("ProjectID = %q, want %q", product.gotReq.ProjectID, testLicenseProjectID)
	}
	if product.gotReq.DeploymentID != testLicenseDeploymentID {
		t.Errorf("DeploymentID = %q, want %q", product.gotReq.DeploymentID, testLicenseDeploymentID)
	}

	var got struct {
		SubscriptionData json.RawMessage `json:"subscriptionData"`
		Signature        string          `json:"signature"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Signature != "sig-value" {
		t.Errorf("Signature = %q, want %q", got.Signature, "sig-value")
	}
	// usageDataPublishingUrl must survive verbatim — it's one of the fields
	// ServiceNow signs, and a closed struct anywhere in this path would drop it.
	if !strings.Contains(string(got.SubscriptionData), "usageDataPublishingUrl") {
		t.Errorf("SubscriptionData = %s, want it to still contain usageDataPublishingUrl", got.SubscriptionData)
	}
}

// TestGetDeploymentLicense_RejectsInvalidUUID confirms the path-parameter
// UUID guard runs before either upstream call.
func TestGetDeploymentLicense_RejectsInvalidUUID(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{}
	entityClient := &fakeProjectAccessChecker{}

	mux := newLicenseMux(product, entityClient)
	req := authedRequest(http.MethodPost, "/projects/not-a-uuid/deployments/"+testLicenseDeploymentID+"/license", "")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if entityClient.calls != 0 || product.calls != 0 {
		t.Fatalf("GetProject calls = %d, ProcessLicenseDownload calls = %d, want 0 and 0", entityClient.calls, product.calls)
	}
}

func newUsageMux(product productConsumptionClient, entityClient entityProjectAccessChecker) *http.ServeMux {
	h := NewProductConsumptionHandler(product, entityClient)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /deployment-usages", h.ImportDeploymentUsage)
	return mux
}

func authedBinaryRequest(method, url string, body []byte, contentType string) *http.Request {
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{
		UserID: "user-1",
		Email:  "customer@example.com",
	}))
}

func TestImportDeploymentUsage_Unauthorized(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{}
	entityClient := &fakeProjectAccessChecker{}

	mux := newUsageMux(product, entityClient)
	req := httptest.NewRequest(http.MethodPost, "/deployment-usages", bytes.NewReader([]byte("zip-data")))
	req.Header.Set("Content-Type", "application/zip")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if product.importCalls != 0 {
		t.Fatalf("importCalls = %d, want 0", product.importCalls)
	}
}

func TestImportDeploymentUsage_RejectsNonZipContentType(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{}
	entityClient := &fakeProjectAccessChecker{}

	mux := newUsageMux(product, entityClient)
	req := authedBinaryRequest(http.MethodPost, "/deployment-usages", []byte("not-a-zip"), "text/plain")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if product.importCalls != 0 {
		t.Fatalf("importCalls = %d, want 0", product.importCalls)
	}
}

func TestImportDeploymentUsage_Success(t *testing.T) {
	msg := "Usage imported successfully."
	product := &fakeLicenseProductConsumptionClient{
		importRes: productconsumption.ImportUsageResponse{
			Message: &msg,
			Result:  map[string]any{"imported": true},
		},
	}
	entityClient := &fakeProjectAccessChecker{visibleDeployments: map[string]bool{"DEP000002442": true}}

	testZipBytes := usageReportZip(t, map[string]string{"usage-report.json": testUsageReport})
	mux := newUsageMux(product, entityClient)
	req := authedBinaryRequest(http.MethodPost, "/deployment-usages", testZipBytes, "application/zip; charset=binary")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if product.importCalls != 1 {
		t.Fatalf("importCalls = %d, want 1", product.importCalls)
	}
	if got := entityClient.gotSearch; len(got.Numbers) != 1 || got.Numbers[0] != "DEP000002442" || !got.IncludeInactive {
		t.Errorf("deployment lookup = %+v, want the report's number, inactive included", got)
	}
	if product.gotImportEmail != "customer@example.com" {
		t.Errorf("gotImportEmail = %q, want %q", product.gotImportEmail, "customer@example.com")
	}
	if !bytes.Equal(product.gotImportZip, testZipBytes) {
		t.Errorf("gotImportZip = %q, want %q", product.gotImportZip, testZipBytes)
	}

	var res struct {
		Message string         `json:"message"`
		Result  map[string]any `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if res.Message != msg {
		t.Errorf("Message = %q, want %q", res.Message, msg)
	}
	if res.Result["imported"] != true {
		t.Errorf("Result[imported] = %v, want true", res.Result["imported"])
	}
}

func TestImportDeploymentUsage_MapsUpstreamError(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{
		importErr: apierror.NewUpstreamError(http.StatusBadGateway, []byte(`{"error":"tracking service unavailable"}`)),
	}
	entityClient := &fakeProjectAccessChecker{visibleDeployments: map[string]bool{"DEP000002442": true}}

	mux := newUsageMux(product, entityClient)
	req := authedBinaryRequest(http.MethodPost, "/deployment-usages", usageReportZip(t, map[string]string{"r.json": testUsageReport}), "application/zip")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// A report for a deployment the caller cannot see is refused before anything is
// forwarded: entity-service's search runs as the caller and returns nothing.
func TestImportDeploymentUsage_RefusesADeploymentTheCallerCannotSee(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{}
	entityClient := &fakeProjectAccessChecker{visibleDeployments: map[string]bool{}}

	mux := newUsageMux(product, entityClient)
	req := authedBinaryRequest(http.MethodPost, "/deployment-usages", usageReportZip(t, map[string]string{"r.json": testUsageReport}), "application/zip")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
	if product.importCalls != 0 {
		t.Fatalf("importCalls = %d, want 0", product.importCalls)
	}
}

func TestImportDeploymentUsage_RefusesWhatIsNotAUsageReport(t *testing.T) {
	cases := map[string][]byte{
		"not a zip":            []byte("PK\x03\x04sample-zip-content"),
		"two files":            usageReportZip(t, map[string]string{"a.json": testUsageReport, "b.json": testUsageReport}),
		"not json":             usageReportZip(t, map[string]string{"report.csv": "a,b"}),
		"no export_metadata":   usageReportZip(t, map[string]string{"r.json": `{"nodeId":"n","dailyUsages":[]}`}),
		"no deployment number": usageReportZip(t, map[string]string{"r.json": `{"export_metadata":{"deployment_id":" "}}`}),
		"malformed json":       usageReportZip(t, map[string]string{"r.json": `{"export_metadata":`}),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			product := &fakeLicenseProductConsumptionClient{}
			entityClient := &fakeProjectAccessChecker{visibleDeployments: map[string]bool{"DEP000002442": true}}
			mux := newUsageMux(product, entityClient)
			req := authedBinaryRequest(http.MethodPost, "/deployment-usages", body, "application/zip")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if entityClient.searchCalls != 0 || product.importCalls != 0 {
				t.Fatalf("searchCalls = %d, importCalls = %d, want 0 and 0", entityClient.searchCalls, product.importCalls)
			}
		})
	}
}

func TestImportDeploymentUsage_DeploymentLookupFailureIsNotForwarded(t *testing.T) {
	product := &fakeLicenseProductConsumptionClient{}
	entityClient := &fakeProjectAccessChecker{searchErr: apierror.NewUpstreamError(http.StatusInternalServerError, nil)}

	mux := newUsageMux(product, entityClient)
	req := authedBinaryRequest(http.MethodPost, "/deployment-usages", usageReportZip(t, map[string]string{"r.json": testUsageReport}), "application/zip")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code < 500 {
		t.Fatalf("status = %d, want a 5xx", rec.Code)
	}
	if product.importCalls != 0 {
		t.Fatalf("importCalls = %d, want 0", product.importCalls)
	}
}
