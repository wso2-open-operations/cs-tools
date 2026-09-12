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
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

var (
	testAttachmentSysid    = sysid32('a')
	testAttachmentRefSysid = sysid32('b')
)

// TestSNCaseService_GetAttachment_NotFound verifies a 404 from the backing service
// surfaces as a NotFoundError.
// TestSNCaseService_GetAttachmentByID_HappyPath proves a well-formed response
// decodes into domain.AttachmentDetails: the id/referenceId sysid->uuid
// mapping, the base64 content, and the createdOn parse.
//
// That parse is why this test matters -- GetAttachmentByID returns an error for
// the whole call if createdOn does not match snCreatedOnLayout, so an upstream
// format change breaks the endpoint outright rather than degrading a field.
func TestSNCaseService_GetAttachmentByID_HappyPath(t *testing.T) {
	attachmentUUID := sysidToUUID(testAttachmentSysid)

	mux := http.NewServeMux()
	mux.HandleFunc("/attachments/"+testAttachmentSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		_, _ = w.Write([]byte(`{
			"id": "` + testAttachmentSysid + `",
			"referenceId": "` + testAttachmentRefSysid + `",
			"name": "logs.txt",
			"type": "text/plain",
			"sizeBytes": 1024,
			"description": "Diagnostic logs",
			"createdBy": "jane.doe@example.com",
			"createdByFullName": "Jane Doe",
			"createdByUser": null,
			"createdOn": "2026-01-01 00:00:00",
			"downloadUrl": "https://example.com/download/1",
			"previewUrl": "https://example.com/preview/1",
			"content": "bG9ncw=="
		}`))
	})

	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, noopSLAClockService{}, nil, "", nil)

	got, err := svc.GetAttachmentByID(contextWithUserIDToken("token"), attachmentUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != attachmentUUID {
		t.Errorf("ID = %q, want %q", got.ID, attachmentUUID)
	}
	if want := sysidToUUID(testAttachmentRefSysid); got.ReferenceID != want {
		t.Errorf("ReferenceID = %q, want %q", got.ReferenceID, want)
	}
	if got.Name != "logs.txt" || got.Type != "text/plain" || got.SizeBytes != 1024 {
		t.Errorf("name/type/size = %q/%q/%d, want logs.txt/text/plain/1024", got.Name, got.Type, got.SizeBytes)
	}
	if got.Description == nil || *got.Description != "Diagnostic logs" {
		t.Errorf("Description = %v, want \"Diagnostic logs\"", got.Description)
	}
	// createdByUser is null on this path, so the bare string is what survives.
	if got.CreatedBy != "jane.doe@example.com" {
		t.Errorf("CreatedBy = %q, want jane.doe@example.com", got.CreatedBy)
	}
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !got.CreatedOn.Equal(want) {
		t.Errorf("CreatedOn = %v, want %v", got.CreatedOn, want)
	}
	if got.Content == nil || *got.Content != "bG9ncw==" {
		t.Errorf("Content = %v, want the base64 body", got.Content)
	}
	if got.DownloadURL == nil || *got.DownloadURL != "https://example.com/download/1" {
		t.Errorf("DownloadURL = %v", got.DownloadURL)
	}
	if got.PreviewURL == nil || *got.PreviewURL != "https://example.com/preview/1" {
		t.Errorf("PreviewURL = %v", got.PreviewURL)
	}
}

// TestSNCaseService_GetAttachmentByID_RejectsUnparseableCreatedOn pins the
// failure mode above: an unexpected createdOn format fails the call loudly
// rather than yielding a zero timestamp the caller would render as 1 Jan 0001.
func TestSNCaseService_GetAttachmentByID_RejectsUnparseableCreatedOn(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/attachments/"+testAttachmentSysid, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"` + testAttachmentSysid + `","createdOn":"2026-01-01T00:00:00Z"}`))
	})

	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, noopSLAClockService{}, nil, "", nil)

	_, err := svc.GetAttachmentByID(contextWithUserIDToken("token"), sysidToUUID(testAttachmentSysid))
	if err == nil {
		t.Fatal("expected an error for an RFC3339 createdOn, got nil")
	}
}

func TestSNCaseService_GetAttachment_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/attachments/"+testAttachmentSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"attachment not found"},"status":"failure"}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	_, err := svc.GetAttachmentByID(contextWithUserIDToken("token"), sysidToUUID(testAttachmentSysid))
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("GetAttachment error = %v (%T), want NotFoundError", err, err)
	}
}

// TestSNCaseService_GetAttachment_RejectsInvalidUUID proves the attachment id is
// validated before any request reaches the backing service.
func TestSNCaseService_GetAttachment_RejectsInvalidUUID(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected call to backing service for an invalid attachment id")
	}))
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	_, err := svc.GetAttachmentByID(contextWithUserIDToken("token"), "not-a-uuid")
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("GetAttachment error = %v (%T), want ValidationError", err, err)
	}
}

// TestSNCaseService_UpdateAttachment_HappyPath proves a valid update request is
// forwarded to SN and the mocked response is parsed back into
// domain.UpdateAttachmentResponse.
func TestSNCaseService_UpdateAttachment_HappyPath(t *testing.T) {
	attachmentUUID := sysidToUUID(testAttachmentSysid)
	refUUID := sysidToUUID(testAttachmentRefSysid)
	newName := "renamed.txt"
	newDescription := "Updated description"

	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/attachments/"+testAttachmentSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{
			"message": "updated",
			"attachment": {
				"id": "` + testAttachmentSysid + `",
				"updatedOn": "2026-02-01 00:00:00",
				"updatedBy": "jane.doe@example.com"
			}
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.UpdateAttachmentRequest{
		AttachmentID:  attachmentUUID,
		ReferenceID:   refUUID,
		ReferenceType: domain.ReferenceTypeDeployment,
		Name:          &newName,
		Description:   json.RawMessage(`"` + newDescription + `"`),
	}

	resp, err := svc.UpdateAttachment(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody["referenceId"] != testAttachmentRefSysid {
		t.Errorf("outbound referenceId = %v, want %q", gotBody["referenceId"], testAttachmentRefSysid)
	}
	if gotBody["referenceType"] != string(domain.ReferenceTypeDeployment) {
		t.Errorf("outbound referenceType = %v, want %q", gotBody["referenceType"], domain.ReferenceTypeDeployment)
	}
	if gotBody["name"] != newName {
		t.Errorf("outbound name = %v, want %q", gotBody["name"], newName)
	}
	if gotBody["description"] != newDescription {
		t.Errorf("outbound description = %v, want %q", gotBody["description"], newDescription)
	}

	if resp.Message != "updated" {
		t.Errorf("Message = %q, want updated", resp.Message)
	}
	if resp.Attachment.ID != attachmentUUID {
		t.Errorf("Attachment.ID = %q, want %q", resp.Attachment.ID, attachmentUUID)
	}
	if resp.Attachment.UpdatedBy != "jane.doe@example.com" {
		t.Errorf("Attachment.UpdatedBy = %q, want jane.doe@example.com", resp.Attachment.UpdatedBy)
	}
}

// TestSNCaseService_UpdateAttachment_DescriptionThreeStates proves the three states a
// caller can express for description are preserved onto the outbound payload: omitted
// (field absent, name alone satisfies "at least one field"), explicit null (clears it),
// and a value (sets it). Before this fix, description was a plain *string, so omitted
// and explicit-null were indistinguishable.
func TestSNCaseService_UpdateAttachment_DescriptionThreeStates(t *testing.T) {
	attachmentUUID := sysidToUUID(testAttachmentSysid)
	refUUID := sysidToUUID(testAttachmentRefSysid)
	name := "renamed.txt"

	cases := []struct {
		name           string
		req            domain.UpdateAttachmentRequest
		wantHasBodyKey bool
		wantBodyValue  any
	}{
		{
			name: "description omitted",
			req: domain.UpdateAttachmentRequest{
				AttachmentID:  attachmentUUID,
				ReferenceID:   refUUID,
				ReferenceType: domain.ReferenceTypeDeployment,
				Name:          &name,
			},
			wantHasBodyKey: false,
		},
		{
			name: "description explicit null",
			req: domain.UpdateAttachmentRequest{
				AttachmentID:  attachmentUUID,
				ReferenceID:   refUUID,
				ReferenceType: domain.ReferenceTypeDeployment,
				Description:   json.RawMessage(`null`),
			},
			wantHasBodyKey: true,
			wantBodyValue:  nil,
		},
		{
			name: "description set to a value",
			req: domain.UpdateAttachmentRequest{
				AttachmentID:  attachmentUUID,
				ReferenceID:   refUUID,
				ReferenceType: domain.ReferenceTypeDeployment,
				Description:   json.RawMessage(`"Updated description"`),
			},
			wantHasBodyKey: true,
			wantBodyValue:  "Updated description",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			mux := http.NewServeMux()
			mux.HandleFunc("/attachments/"+testAttachmentSysid, func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				_, _ = w.Write([]byte(`{
					"message": "updated",
					"attachment": {
						"id": "` + testAttachmentSysid + `",
						"updatedOn": "2026-02-01 00:00:00",
						"updatedBy": "jane.doe@example.com"
					}
				}`))
			})

			client := newTestSNClient(t, mux)
			svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

			if _, err := svc.UpdateAttachment(contextWithUserIDToken("token"), tc.req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotValue, gotHasKey := gotBody["description"]
			if gotHasKey != tc.wantHasBodyKey {
				t.Fatalf("outbound body has %q key = %v, want %v (body: %v)", "description", gotHasKey, tc.wantHasBodyKey, gotBody)
			}
			if gotHasKey && gotValue != tc.wantBodyValue {
				t.Errorf("outbound description = %v, want %v", gotValue, tc.wantBodyValue)
			}
		})
	}
}

// TestSNCaseService_UpdateAttachment_ValidationErrors covers the three validation
// failures UpdateAttachment must catch before ever reaching the backing service:
// an invalid attachment id, an invalid referenceId, an invalid referenceType, and
// a request providing neither name nor description.
func TestSNCaseService_UpdateAttachment_ValidationErrors(t *testing.T) {
	validAttachmentUUID := sysidToUUID(testAttachmentSysid)
	validRefUUID := sysidToUUID(testAttachmentRefSysid)
	name := "renamed.txt"

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected call to backing service for an invalid request")
	}))
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	cases := []struct {
		name string
		req  domain.UpdateAttachmentRequest
	}{
		{
			name: "invalid attachment id",
			req: domain.UpdateAttachmentRequest{
				AttachmentID:  "not-a-uuid",
				ReferenceID:   validRefUUID,
				ReferenceType: domain.ReferenceTypeDeployment,
				Name:          &name,
			},
		},
		{
			name: "invalid reference id",
			req: domain.UpdateAttachmentRequest{
				AttachmentID:  validAttachmentUUID,
				ReferenceID:   "not-a-uuid",
				ReferenceType: domain.ReferenceTypeDeployment,
				Name:          &name,
			},
		},
		{
			name: "invalid reference type",
			req: domain.UpdateAttachmentRequest{
				AttachmentID:  validAttachmentUUID,
				ReferenceID:   validRefUUID,
				ReferenceType: domain.ReferenceType("not_a_type"),
				Name:          &name,
			},
		},
		{
			name: "neither name nor description provided",
			req: domain.UpdateAttachmentRequest{
				AttachmentID:  validAttachmentUUID,
				ReferenceID:   validRefUUID,
				ReferenceType: domain.ReferenceTypeDeployment,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateAttachment(contextWithUserIDToken("token"), tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("error = %v (%T), want ValidationError", err, err)
			}
		})
	}
}
