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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestJSONInt accepts both spellings of a whole number and still rejects a
// genuinely fractional one.
func TestJSONInt(t *testing.T) {
	tests := []struct {
		name, in string
		want     jsonInt
		wantErr  bool
	}{
		{"a plain integer", `5`, 5, false},
		{"the same value with a decimal point", `5.0`, 5, false},
		{"zero", `0`, 0, false},
		{"zero as a float", `0.0`, 0, false},
		{"exponent notation for a whole number", `5e0`, 5, false},
		{"a fraction is not a status", `5.5`, 0, true},
		{"a string is not a number", `"5"`, 0, true},
		{"null leaves the value alone, per the encoding/json convention", `null`, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got jsonInt
			err := json.Unmarshal([]byte(tt.in), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal(%s) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("Unmarshal(%s) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestProcessLicenseDownload_AcceptsFloatStatus is the regression test for the
// live failure: the licensing service replied {"result":{"status":5.0}} and the
// whole response failed to decode with
//
//	response decode failed: field "result.status" expects int, got number 5.0
//
// so a fully provisioned project was indistinguishable from one the service had
// never heard of.
func TestProcessLicenseDownload_AcceptsFloatStatus(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	var licenceCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path[len(r.URL.Path)-len("/consumption/status"):] == "/consumption/status":
			// 5.0, exactly as the upstream sends it.
			_, _ = w.Write([]byte(`{"result":{"status":5.0,"applicationId":"app-1"}}`))
		default:
			licenceCalled = true
			_, _ = w.Write([]byte(`{"result":{"license":{}}}`))
		}
	}))
	defer srv.Close()

	c := NewClient(Config{
		SubscriptionBaseURL: srv.URL, TokenURL: tokenSrv.URL,
		ClientID: "id", ClientSecret: "secret",
	})

	if _, err := c.ProcessLicenseDownload(context.Background(), LicenseDownloadRequest{
		Email:        "someone@wso2.com",
		ProjectID:    "6fa0b42d-1bfa-a694-a002-c9d3604bcb77",
		DeploymentID: "937bd77b-1ba0-8750-a002-c9d3604bcbbc",
	}); err != nil {
		t.Fatalf("ProcessLicenseDownload with status 5.0: %v", err)
	}
	if !licenceCalled {
		t.Error("status 5.0 did not reach the licence call; it should mean generated-secret-keys")
	}
}
