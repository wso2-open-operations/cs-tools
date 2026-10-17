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

package productconsumption

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newLicenseTestServers starts a token server and a licensing server that
// answers the license call with the given response body.
func newLicenseTestServers(t *testing.T, licenseBody string) *Client {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(licenseBody))
	}))
	t.Cleanup(srv.Close)

	return NewClient(Config{
		SubscriptionBaseURL: srv.URL, TokenURL: tokenSrv.URL,
		ClientID: "id", ClientSecret: "secret",
	})
}

// TestProcessLicenseDownload_RejectsUnsuccessfulLicense is the regression test
// for the licensing service's refusal shape: a 200 carrying success:false
// must not be handed to the caller as a usable license.
func TestProcessLicenseDownload_RejectsUnsuccessfulLicense(t *testing.T) {
	c := newLicenseTestServers(t, `{"result":{"success":false,"license":{}}}`)

	if _, err := c.ProcessLicenseDownload(context.Background(), LicenseDownloadRequest{
		Email: "someone@wso2.com", ProjectID: "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
		DeploymentID: "937bd77b-1ba0-8750-a002-c9d3604bcbbc",
	}); err == nil {
		t.Fatal("expected an error for success:false, got nil")
	}
}

// TestProcessLicenseDownload_RejectsMissingSubscriptionData is the regression
// test for a success:true response whose subscriptionData is absent or not a
// JSON object — a signature with nothing behind it verifies nothing.
func TestProcessLicenseDownload_RejectsMissingSubscriptionData(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"no subscriptionData at all", `{"result":{"success":true,"license":{"signature":"sig"}}}`},
		{"subscriptionData is null", `{"result":{"success":true,"license":{"subscriptionData":null,"signature":"sig"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newLicenseTestServers(t, tt.body)
			if _, err := c.ProcessLicenseDownload(context.Background(), LicenseDownloadRequest{
				Email: "someone@wso2.com", ProjectID: "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
				DeploymentID: "937bd77b-1ba0-8750-a002-c9d3604bcbbc",
			}); err == nil {
				t.Fatalf("expected an error for %s, got nil", tt.name)
			}
		})
	}
}

// TestProcessLicenseDownload_RejectsMissingSignature is the regression test
// for a success:true response with a well-formed subscriptionData object but
// no signature — the customer's product has nothing to verify the data
// against, the same failure shape as no subscriptionData at all.
func TestProcessLicenseDownload_RejectsMissingSignature(t *testing.T) {
	c := newLicenseTestServers(t, `{"result":{"success":true,"license":{"subscriptionData":{"clientId":"abc"}}}}`)

	if _, err := c.ProcessLicenseDownload(context.Background(), LicenseDownloadRequest{
		Email: "someone@wso2.com", ProjectID: "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
		DeploymentID: "937bd77b-1ba0-8750-a002-c9d3604bcbbc",
	}); err == nil {
		t.Fatal("expected an error for a missing signature, got nil")
	}
}

// TestProcessLicenseDownload_AcceptsValidLicense is the positive counterpart:
// a genuine success:true response with an object subscriptionData must still
// pass through unchanged.
func TestProcessLicenseDownload_AcceptsValidLicense(t *testing.T) {
	c := newLicenseTestServers(t, `{"result":{"success":true,"license":{"subscriptionData":{"clientId":"abc"},"signature":"sig"}}}`)

	license, err := c.ProcessLicenseDownload(context.Background(), LicenseDownloadRequest{
		Email: "someone@wso2.com", ProjectID: "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
		DeploymentID: "937bd77b-1ba0-8750-a002-c9d3604bcbbc",
	})
	if err != nil {
		t.Fatalf("ProcessLicenseDownload: %v", err)
	}
	if license.Signature != "sig" {
		t.Errorf("Signature = %q, want %q", license.Signature, "sig")
	}
	if len(license.SubscriptionData) == 0 {
		t.Error("SubscriptionData is empty, want the passed-through object")
	}
}
