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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestImportDeploymentUsage_SendsBase64Zip(t *testing.T) {
	const (
		testEmail   = "customer@example.com"
		testZipData = "PK\x03\x04sample-deployment-usage-zip-bytes"
	)

	var gotBody ImportUsageRequest
	var gotPath string

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-123","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	trackingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"Usage imported successfully.","result":{"success":true}}`))
	}))
	defer trackingSrv.Close()

	client := NewClient(Config{
		TrackingBaseURL: trackingSrv.URL,
		TokenURL:        tokenSrv.URL,
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
	})

	res, err := client.ImportDeploymentUsage(context.Background(), testEmail, []byte(testZipData))
	if err != nil {
		t.Fatalf("ImportDeploymentUsage: %v", err)
	}

	if gotPath != "/deployment-usages" {
		t.Errorf("path = %q, want %q", gotPath, "/deployment-usages")
	}
	if gotBody.Email != testEmail {
		t.Errorf("Email = %q, want %q", gotBody.Email, testEmail)
	}
	decodedZip, err := base64.StdEncoding.DecodeString(gotBody.Zip)
	if err != nil {
		t.Fatalf("decode base64 zip: %v", err)
	}
	if string(decodedZip) != testZipData {
		t.Errorf("decoded zip = %q, want %q", string(decodedZip), testZipData)
	}
	if res.Message == nil || *res.Message != "Usage imported successfully." {
		t.Errorf("Message = %v, want %q", res.Message, "Usage imported successfully.")
	}
	resMap, ok := res.Result.(map[string]any)
	if !ok || resMap["success"] != true {
		t.Errorf("Result = %v, want map with success: true", res.Result)
	}
}

func TestImportDeploymentUsage_HandlesServerError(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-123","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	trackingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal service unavailable", http.StatusServiceUnavailable)
	}))
	defer trackingSrv.Close()

	client := NewClient(Config{
		TrackingBaseURL: trackingSrv.URL,
		TokenURL:        tokenSrv.URL,
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
	})

	if _, err := client.ImportDeploymentUsage(context.Background(), "user@example.com", []byte("zip-data")); err == nil {
		t.Fatal("expected error on 503 Service Unavailable, got nil")
	}
}
