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

package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

const (
	testSmartAlertUUID       = "99999999-9999-9999-9999-999999999999"
	testSmartAlertSysid      = "99999999999999999999999999999999"
	testSmartAlertAlertUUID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	testSmartAlertAlertSysid = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

// TestSNSmartAlertService_GetSmartAlertByID_MapsFields verifies that a successful
// backing-service response is mapped to the domain view with sysids converted to UUIDs,
// and that opaque fields like details/sourceAlertId pass through unparsed.
func TestSNSmartAlertService_GetSmartAlertByID_MapsFields(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/smart-alert-buffers/"+testSmartAlertSysid {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"sysId": "` + testSmartAlertSysid + `",
			"alertId": "` + testSmartAlertAlertSysid + `",
			"sourceAlertId": "ext-12345",
			"alertStatus": "OPEN",
			"windowStatus": "ACTIVE",
			"severity": "high",
			"urgency": "high",
			"impact": "high",
			"category": "performance",
			"source": "monitor",
			"environment": "production",
			"resourceName": "app-server-1",
			"shortDescription": "high cpu",
			"details": "raw opaque blob {not json safe",
			"monitorUrl": "https://monitor.example.com/d/1",
			"firedAt": "2026-01-01 00:00:00",
			"receivedAt": "2026-01-01 00:00:05",
			"fireCount": 3,
			"incidentSysId": null
		}`))
	}))

	svc := NewServiceNowSmartAlertService(client)

	got, err := svc.GetSmartAlertByID(contextWithUserIDToken("token"), testSmartAlertUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == nil || *got.ID != testSmartAlertUUID {
		t.Errorf("ID = %v, want %q", got.ID, testSmartAlertUUID)
	}
	if got.AlertID == nil || *got.AlertID != testSmartAlertAlertUUID {
		t.Errorf("AlertID = %v, want %q", got.AlertID, testSmartAlertAlertUUID)
	}
	if got.SourceAlertID == nil || *got.SourceAlertID != "ext-12345" {
		t.Errorf("SourceAlertID = %v, want ext-12345 (opaque, unconverted)", got.SourceAlertID)
	}
	if got.Details == nil || *got.Details != "raw opaque blob {not json safe" {
		t.Errorf("Details = %v, want passthrough of the opaque string", got.Details)
	}
	if got.MonitorURL == nil || *got.MonitorURL != "https://monitor.example.com/d/1" {
		t.Errorf("MonitorURL = %v, want the monitor URL", got.MonitorURL)
	}
	if got.FireCount == nil || *got.FireCount != 3 {
		t.Errorf("FireCount = %v, want 3", got.FireCount)
	}
	if got.IncidentID != nil {
		t.Errorf("IncidentID = %v, want nil", got.IncidentID)
	}
}

// TestSNSmartAlertService_GetSmartAlertByID_NotFound verifies that a 404 from the backing
// service, returned in its {"error":{"message":...}} envelope, surfaces as a NotFoundError.
func TestSNSmartAlertService_GetSmartAlertByID_NotFound(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"smart alert not found","detail":"no record"},"status":"failure"}`))
	}))

	svc := NewServiceNowSmartAlertService(client)

	_, err := svc.GetSmartAlertByID(contextWithUserIDToken("token"), testSmartAlertUUID)
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("GetSmartAlertByID error = %v, want NotFoundError", err)
	}
}

// TestSNSmartAlertService_GetSmartAlertByID_InvalidUUID verifies invalid input is
// rejected before any request reaches the backing service.
func TestSNSmartAlertService_GetSmartAlertByID_InvalidUUID(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to %s", r.URL.Path)
	}))

	svc := NewServiceNowSmartAlertService(client)

	_, err := svc.GetSmartAlertByID(contextWithUserIDToken("token"), "not-a-uuid")
	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("GetSmartAlertByID error = %v, want ValidationError", err)
	}
}
