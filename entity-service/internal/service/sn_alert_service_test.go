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
	testAlertUUID     = "77777777-7777-7777-7777-777777777777"
	testAlertSysid    = "77777777777777777777777777777777"
	testAlertIncUUID  = "88888888-8888-8888-8888-888888888888"
	testAlertIncSysid = "88888888888888888888888888888888"
)

// TestSNAlertService_GetAlertByID_MapsFields verifies that a successful backing-service
// response is mapped to the domain view with sysids converted to UUIDs.
func TestSNAlertService_GetAlertByID_MapsFields(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/custom-alerts/"+testAlertSysid {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"sysId": "` + testAlertSysid + `",
			"number": "ALT0001",
			"environment": "production",
			"metricName": "cpu_usage",
			"source": "monitor",
			"category": "performance",
			"severity": "high",
			"description": "cpu spiked",
			"incidentSysId": "` + testAlertIncSysid + `",
			"serviceSysId": null,
			"createdOn": "2026-01-01 00:00:00"
		}`))
	}))

	svc := NewServiceNowAlertService(client)

	got, err := svc.GetAlertByID(contextWithUserIDToken("token"), testAlertUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == nil || *got.ID != testAlertUUID {
		t.Errorf("ID = %v, want %q", got.ID, testAlertUUID)
	}
	if got.Number == nil || *got.Number != "ALT0001" {
		t.Errorf("Number = %v, want ALT0001", got.Number)
	}
	if got.IncidentID == nil || *got.IncidentID != testAlertIncUUID {
		t.Errorf("IncidentID = %v, want %q", got.IncidentID, testAlertIncUUID)
	}
	if got.ServiceID != nil {
		t.Errorf("ServiceID = %v, want nil", got.ServiceID)
	}
	if got.CreatedOn != "2026-01-01 00:00:00" {
		t.Errorf("CreatedOn = %q, want 2026-01-01 00:00:00", got.CreatedOn)
	}
}

// TestSNAlertService_GetAlertByID_NotFound verifies that a 404 from the backing service,
// returned in its {"error":{"message":...}} envelope, surfaces as a NotFoundError.
func TestSNAlertService_GetAlertByID_NotFound(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"alert not found","detail":"no record"},"status":"failure"}`))
	}))

	svc := NewServiceNowAlertService(client)

	_, err := svc.GetAlertByID(contextWithUserIDToken("token"), testAlertUUID)
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("GetAlertByID error = %v, want NotFoundError", err)
	}
}

// TestSNAlertService_GetAlertByID_InvalidUUID verifies invalid input is rejected before
// any request reaches the backing service.
func TestSNAlertService_GetAlertByID_InvalidUUID(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to %s", r.URL.Path)
	}))

	svc := NewServiceNowAlertService(client)

	_, err := svc.GetAlertByID(contextWithUserIDToken("token"), "not-a-uuid")
	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("GetAlertByID error = %v, want ValidationError", err)
	}
}
