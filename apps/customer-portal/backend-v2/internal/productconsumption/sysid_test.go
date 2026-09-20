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
	"strings"
	"testing"
)

func TestUUIDToSysID(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"a dashed uuid loses its hyphens",
			"6fa0b42d-1bfa-a694-a002-c9d3604bcb77", "6fa0b42d1bfaa694a002c9d3604bcb77"},
		{"a bare sysid is already right and must not change",
			"6fa0b42d1bfaa694a002c9d3604bcb77", "6fa0b42d1bfaa694a002c9d3604bcb77"},
		{"uppercase hex still converts",
			"6FA0B42D-1BFA-A694-A002-C9D3604BCB77", "6FA0B42D1BFAA694A002C9D3604BCB77"},
		{"empty stays empty", "", ""},
		{"a non-uuid passes through untouched", "not-an-id", "not-an-id"},
		{"right length, wrong hyphen positions, left alone",
			"6fa0b42d1-bfa-a694-a002-c9d3604bcb7", "6fa0b42d1-bfa-a694-a002-c9d3604bcb7"},
		{"right shape but a non-hex character, left alone",
			"6fa0b42d-1bfa-a694-a002-c9d3604bcbZZ", "6fa0b42d-1bfa-a694-a002-c9d3604bcbZZ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uuidToSysID(tt.in); got != tt.want {
				t.Errorf("uuidToSysID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestLicenceCallsSendBareSysIDs is the regression test for the bug: the portal
// deals in dashed UUIDs (entity-service converts sysids on the way out), and
// the product-consumption service identifies projects and deployments by bare
// sysids. Sending a dashed id matched nothing there, so the call succeeded and
// described a project that does not exist.
//
// Asserts on the URLs the client actually builds, through a real test server.
func TestLicenceCallsSendBareSysIDs(t *testing.T) {
	const (
		dashedProject = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
		bareProject   = "6fa0b42d1bfaa694a002c9d3604bcb77"
		dashedDeploy  = "937bd77b-1ba0-8750-a002-c9d3604bcbbc"
		bareDeploy    = "937bd77b1ba08750a002c9d3604bcbbc"
	)

	var paths []string
	var statusBody map[string]any

	// The client authenticates with client credentials, so it needs a token
	// endpoint before it will make any call at all.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/consumption/status") {
			_ = json.NewDecoder(r.Body).Decode(&statusBody)
			// status 4 == generated secret keys, so the flow goes straight to
			// the licence call without provisioning anything.
			_, _ = w.Write([]byte(`{"result":{"status":4,"applicationId":"app-1"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"license":{}}}`))
	}))
	defer srv.Close()

	c := NewClient(Config{
		SubscriptionBaseURL: srv.URL,
		TokenURL:            tokenSrv.URL,
		ClientID:            "id",
		ClientSecret:        "secret",
	})
	if _, err := c.ProcessLicenseDownload(context.Background(), LicenseDownloadRequest{
		Email: "someone@wso2.com", ProjectID: dashedProject, DeploymentID: dashedDeploy,
	}); err != nil {
		t.Fatalf("ProcessLicenseDownload: %v", err)
	}

	for _, p := range paths {
		if strings.Contains(p, dashedProject) || strings.Contains(p, dashedDeploy) {
			t.Errorf("a dashed id reached the upstream path: %s", p)
		}
	}
	joined := strings.Join(paths, " ")
	if !strings.Contains(joined, bareProject) {
		t.Errorf("no bare project sysid in any path. paths: %v", paths)
	}
	if !strings.Contains(joined, bareDeploy) {
		t.Errorf("no bare deployment sysid in the licence path. paths: %v", paths)
	}
	if got, _ := statusBody["deploymentId"].(string); got != bareDeploy {
		t.Errorf("consumption-status body deploymentId = %q, want the bare sysid %q", got, bareDeploy)
	}
}
