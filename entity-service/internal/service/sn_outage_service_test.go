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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	testOutageUUID  = "77777777-7777-7777-7777-777777777777"
	testOutageSysid = "77777777777777777777777777777777"
)

func TestSNOutageService_CreateOutage_ValidatesRequiredFields(t *testing.T) {
	client := newTestSNClient(t, http.NewServeMux())
	svc := NewServiceNowOutageService(client)

	tests := []struct {
		name string
		req  domain.CreateOutageRequest
	}{
		{"missing type", domain.CreateOutageRequest{Begin: "2026-08-18 09:00:00", ShortDescription: "test"}},
		{"invalid type", domain.CreateOutageRequest{Type: "catastrophe", Begin: "2026-08-18 09:00:00", ShortDescription: "test"}},
		{"missing begin", domain.CreateOutageRequest{Type: domain.OutageTypeOutage, ShortDescription: "test"}},
		{"missing shortDescription", domain.CreateOutageRequest{Type: domain.OutageTypeOutage, Begin: "2026-08-18 09:00:00"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOutage(contextWithUserIDToken("token"), tc.req)
			var ve *apierror.ValidationError
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !isValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// isValidationError is a small helper so the table test above stays readable.
func isValidationError(err error, target **apierror.ValidationError) bool {
	ve, ok := err.(*apierror.ValidationError)
	if ok {
		*target = ve
	}
	return ok
}

func TestSNOutageService_CreateOutage_Success(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/outages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Outage created successfully",
			"outage": {
				"id": "` + testOutageSysid + `", "number": "OUT0001881",
				"type": "degradation", "status": "resolved",
				"begin": "2026-08-18 09:00:00", "end": "2026-08-18 10:30:00", "duration": "1970-01-01 01:30:00",
				"shortDescription": "test outage",
				"configurationItem": null, "incident": null,
				"affectedConfigurationItems": [],
				"publishesToStatusPage": false, "statusPageCloud": null,
				"createdOn": "2026-08-18 09:00:00", "createdBy": "engineer@example.com",
				"updatedOn": "2026-08-18 09:00:00", "updatedBy": "engineer@example.com"
			}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOutageService(client)

	req := domain.CreateOutageRequest{
		Type:             domain.OutageTypeDegradation,
		Begin:            "2026-08-18 09:00:00",
		ShortDescription: "test outage",
	}
	resp, err := svc.CreateOutage(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Outage.ID != sysidToUUID(testOutageSysid) {
		t.Errorf("outage.id: got %q, want %q", resp.Outage.ID, sysidToUUID(testOutageSysid))
	}
	if resp.Outage.Number != "OUT0001881" {
		t.Errorf("outage.number: got %q", resp.Outage.Number)
	}
	if resp.Outage.PublishesToStatusPage {
		t.Error("expected publishesToStatusPage to be false")
	}
	if gotType, _ := gotBody["type"].(string); gotType != "degradation" {
		t.Errorf("sent type: got %q, want degradation", gotType)
	}
}

// TestSNOutageService_CreateOutage_PublicationFieldsPassThrough verifies that
// publishesToStatusPage/statusPageCloud are surfaced exactly as the backing
// service returns them and are never recomputed locally -- see
// CHANGES-outage-api.md: these are the only signal that an outage is publicly
// visible, and a defect here would silently mis-tell the caller.
func TestSNOutageService_CreateOutage_PublicationFieldsPassThrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/outages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Outage created successfully",
			"outage": {
				"id": "` + testOutageSysid + `", "number": "OUT0001882",
				"type": "outage", "status": "in_progress",
				"begin": "2026-08-18 09:00:00", "end": null, "duration": null,
				"shortDescription": "test outage",
				"configurationItem": {"id": "` + testOutageSysid + `", "name": "Asgardeo Service - US", "className": "service_offering"},
				"incident": null,
				"affectedConfigurationItems": [],
				"publishesToStatusPage": true, "statusPageCloud": "asgardeo",
				"createdOn": "2026-08-18 09:00:00", "createdBy": "engineer@example.com",
				"updatedOn": "2026-08-18 09:00:00", "updatedBy": "engineer@example.com"
			}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOutageService(client)

	ackTrue := true
	ciID := testOutageUUID
	req := domain.CreateOutageRequest{
		Type: domain.OutageTypeOutage, Begin: "2026-08-18 09:00:00", ShortDescription: "test outage",
		ConfigurationItemID: &ciID, AcknowledgePublicPublication: &ackTrue,
	}
	resp, err := svc.CreateOutage(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Outage.PublishesToStatusPage {
		t.Error("expected publishesToStatusPage to be true, passed through unchanged from the backing service")
	}
	if resp.Outage.StatusPageCloud == nil || *resp.Outage.StatusPageCloud != "asgardeo" {
		t.Errorf("statusPageCloud: got %v, want \"asgardeo\"", resp.Outage.StatusPageCloud)
	}
}

func TestSNOutageService_UpdateOutage_EndNullReopens(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/outages/"+testOutageSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Outage updated successfully",
			"outage": {
				"id": "` + testOutageSysid + `", "number": "OUT0001881",
				"type": "outage", "status": "in_progress",
				"begin": "2026-08-18 09:00:00", "end": null, "duration": null,
				"shortDescription": "test outage",
				"configurationItem": null, "incident": null,
				"affectedConfigurationItems": [],
				"publishesToStatusPage": false, "statusPageCloud": null,
				"createdOn": "2026-08-18 09:00:00", "createdBy": "engineer@example.com",
				"updatedOn": "2026-08-18 09:05:00", "updatedBy": "engineer@example.com"
			}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOutageService(client)

	var endOuter *string // outer non-nil, inner nil == explicit null (reopen)
	req := domain.PatchOutageRequest{ID: testOutageUUID, End: &endOuter}

	resp, err := svc.UpdateOutage(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Outage.End != nil {
		t.Errorf("expected outage.end to be nil after reopen, got %v", resp.Outage.End)
	}

	raw, ok := gotBody["end"]
	if !ok {
		t.Fatal("expected \"end\" key to be present in the outgoing payload (explicit null), but it was omitted")
	}
	if raw != nil {
		t.Errorf("expected \"end\" to be sent as JSON null, got %v", raw)
	}
}

func TestSNOutageService_UpdateOutage_NoFieldsIsValidationError(t *testing.T) {
	client := newTestSNClient(t, http.NewServeMux())
	svc := NewServiceNowOutageService(client)

	_, err := svc.UpdateOutage(contextWithUserIDToken("token"), domain.PatchOutageRequest{ID: testOutageUUID})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestSNOutageService_AddOutageCommunication_ExternalIsPublic(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/outages/"+testOutageSysid+"/communications", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Communication added successfully",
			"communication": {
				"id": "` + testOutageSysid + `", "channel": "external", "body": "status update",
				"isPublic": true, "createdOn": "2026-08-18 09:00:00", "createdBy": "engineer@example.com"
			}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOutageService(client)

	req := domain.AddOutageCommunicationRequest{
		OutageID: testOutageUUID, Channel: domain.OutageCommunicationChannelExternal, Body: "status update",
	}
	resp, err := svc.AddOutageCommunication(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Communication.IsPublic {
		t.Error("expected external channel communication to be public")
	}
}

func TestSNOutageService_AddOutageCommunication_InvalidChannel(t *testing.T) {
	client := newTestSNClient(t, http.NewServeMux())
	svc := NewServiceNowOutageService(client)

	req := domain.AddOutageCommunicationRequest{OutageID: testOutageUUID, Channel: "twitter", Body: "hi"}
	_, err := svc.AddOutageCommunication(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestSNOutageService_SearchOutageCommunications_FlatPayload(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/outages/"+testOutageSysid+"/communications/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"communications": [], "offset": 0, "limit": 20, "totalRecords": 0
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOutageService(client)

	req := domain.SearchOutageCommunicationsRequest{
		OutageID: testOutageUUID,
		Channels: []domain.OutageCommunicationChannel{domain.OutageCommunicationChannelExternal},
	}
	if _, err := svc.SearchOutageCommunications(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The outgoing payload must be flat -- Choreo's OutageCommunicationSearchPayload is a closed
	// record with no pagination wrapper; a nested shape 400s (confirmed live).
	if _, ok := gotBody["pagination"]; ok {
		t.Fatal("outgoing payload must not nest fields under \"pagination\" -- Choreo's OutageCommunicationSearchPayload is flat")
	}
	channels, ok := gotBody["channels"].([]any)
	if !ok || len(channels) != 1 || channels[0] != "external" {
		t.Errorf("expected top-level channels == [\"external\"], got %v", gotBody["channels"])
	}
	if _, ok := gotBody["limit"]; !ok {
		t.Error("expected top-level \"limit\" in outgoing payload")
	}
}

func TestSNOutageService_SearchOutages_MapsFiltersAndResponse(t *testing.T) {
	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/outages/search", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"outages": [],
			"offset": 0, "limit": 20, "totalRecords": 0,
			"appliedBeginFrom": "2026-02-18 00:00:00", "beginFromDefaulted": true
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOutageService(client)

	req := domain.SearchOutagesRequest{Filters: domain.SearchOutagesFilters{Types: []domain.OutageType{domain.OutageTypeOutage}}}
	resp, err := svc.SearchOutages(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.BeginFromDefaulted {
		t.Error("expected beginFromDefaulted to be true")
	}
	if resp.AppliedBeginFrom != "2026-02-18 00:00:00" {
		t.Errorf("appliedBeginFrom: got %q", resp.AppliedBeginFrom)
	}

	// The outgoing payload must be flat -- Choreo's OutageSearchPayload is a
	// closed record with no filters/pagination wrapper; a nested shape 400s.
	if _, ok := gotBody["filters"]; ok {
		t.Fatal("outgoing payload must not nest fields under \"filters\" -- Choreo's OutageSearchPayload is flat")
	}
	if _, ok := gotBody["pagination"]; ok {
		t.Fatal("outgoing payload must not nest fields under \"pagination\" -- Choreo's OutageSearchPayload is flat")
	}
	types, ok := gotBody["types"].([]any)
	if !ok || len(types) != 1 || types[0] != "outage" {
		t.Errorf("expected top-level types == [\"outage\"], got %v", gotBody["types"])
	}
}

func TestSNOutageService_SearchOutages_InvalidType(t *testing.T) {
	client := newTestSNClient(t, http.NewServeMux())
	svc := NewServiceNowOutageService(client)

	req := domain.SearchOutagesRequest{Filters: domain.SearchOutagesFilters{Types: []domain.OutageType{"catastrophe"}}}
	_, err := svc.SearchOutages(contextWithUserIDToken("token"), req)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestSNOutageService_GetOutageByID_MapsCommunicationCounts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/outages/"+testOutageSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		// The real response wraps the detail object under "outage" (OutageResponse ->
		// OutageDetails) -- this shape caught a real bug where an earlier flat mock let
		// an unwrap-less struct silently unmarshal to all zero values with no error.
		_, _ = w.Write([]byte(`{
			"outage": {
				"id": "` + testOutageSysid + `", "number": "OUT0001875",
				"type": "outage", "status": "resolved",
				"begin": "2026-08-18 09:00:00", "end": "2026-08-18 10:00:00", "duration": "1970-01-01 01:00:00",
				"shortDescription": "test outage",
				"configurationItem": null, "incident": null,
				"affectedConfigurationItems": [],
				"publishesToStatusPage": false, "statusPageCloud": null,
				"createdOn": "2026-08-18 09:00:00", "createdBy": "engineer@example.com",
				"updatedOn": "2026-08-18 09:00:00", "updatedBy": "engineer@example.com",
				"communicationCounts": {"external": 2, "internal": 2, "additional": 0}
			}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOutageService(client)

	resp, err := svc.GetOutageByID(contextWithUserIDToken("token"), testOutageUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Outage.Number != "OUT0001875" {
		t.Errorf("outage.number: got %q, want %q -- struct must unwrap the \"outage\" key, not read it flat", resp.Outage.Number, "OUT0001875")
	}
	if resp.CommunicationCounts.External != 2 || resp.CommunicationCounts.Internal != 2 || resp.CommunicationCounts.Additional != 0 {
		t.Errorf("communicationCounts: got %+v", resp.CommunicationCounts)
	}
}

func TestSNOutageService_GetOutageMetadata(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/outages/metadata", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"types": [{"value": "outage", "label": "Outage"}, {"value": "degradation", "label": "Degradation"}, {"value": "planned", "label": "Planned"}],
			"statuses": [{"value": "in_progress", "label": "In Progress"}, {"value": "resolved", "label": "Resolved"}],
			"communicationChannels": [{"value": "external", "label": "External", "isPublic": true}, {"value": "internal", "label": "Internal", "isPublic": false}, {"value": "additional", "label": "Additional", "isPublic": false}],
			"statusPageClouds": ["asgardeo", "bijira", "choreo", "choreo-eu", "devant", "moesif"]
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOutageService(client)

	resp, err := svc.GetOutageMetadata(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Types) != 3 {
		t.Errorf("expected 3 types, got %d", len(resp.Types))
	}
	if resp.Types[0].Value != "outage" || resp.Types[0].Label != "Outage" {
		t.Errorf("unexpected first type: %+v", resp.Types[0])
	}
	if len(resp.CommunicationChannels) != 3 {
		t.Errorf("expected 3 communication channels, got %d", len(resp.CommunicationChannels))
	}
	if resp.CommunicationChannels[0].Value != "external" || !resp.CommunicationChannels[0].IsPublic {
		t.Errorf("unexpected first channel: %+v", resp.CommunicationChannels[0])
	}
	if len(resp.StatusPageClouds) != 6 {
		t.Errorf("expected 6 status page clouds, got %d", len(resp.StatusPageClouds))
	}
}
