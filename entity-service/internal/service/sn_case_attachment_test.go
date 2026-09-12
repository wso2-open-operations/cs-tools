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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// testSNAttachmentSysid is a ServiceNow attachment sysid used across the
// attachment-response tests below.
var testSNAttachmentSysid = sysid32('1')

// TestSNCaseService_GetAttachmentByID_StatusAndFields proves a ServiceNow
// attachment-details response always reports status "complete" (SN's
// /attachments API only ever returns fully-uploaded files -- there is no
// pending state on this data source), and that DownloadURL/Content -- both
// pointer fields shared with the Postgres-backed path -- are populated
// (non-nil) here, since ServiceNow does hold a real download URL and content
// for every attachment it returns.
func TestSNCaseService_GetAttachmentByID_StatusAndFields(t *testing.T) {
	body := `{
		"id": "` + testSNAttachmentSysid + `",
		"referenceId": "` + testWLCaseSysid + `",
		"name": "diagnostics.log",
		"type": "text/plain",
		"sizeBytes": 2048,
		"createdBy": "agent.smith",
		"createdOn": "2026-01-01 10:00:00",
		"downloadUrl": "https://sn.example.com/download/1",
		"content": "ZmFrZSBjb250ZW50"
	}`

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	details, err := svc.GetAttachmentByID(contextWithUserIDToken("token"), sysidToUUID(testSNAttachmentSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if details.Status != domain.AttachmentStatusComplete {
		t.Fatalf("expected status %q, got %q", domain.AttachmentStatusComplete, details.Status)
	}
	if details.DownloadURL == nil || *details.DownloadURL != "https://sn.example.com/download/1" {
		t.Fatalf("expected a non-nil downloadUrl, got %v", details.DownloadURL)
	}
	if details.Content == nil || *details.Content != "ZmFrZSBjb250ZW50" {
		t.Fatalf("expected non-nil content, got %v", details.Content)
	}
}

// TestSNCaseService_CreateCaseAttachment_StatusComplete proves the
// create-attachment response also reports status "complete" for the
// ServiceNow data source, and a non-nil downloadUrl.
func TestSNCaseService_CreateCaseAttachment_StatusComplete(t *testing.T) {
	body := `{
		"message": "Attachment created successfully",
		"attachment": {
			"id": "` + testSNAttachmentSysid + `",
			"sizeBytes": 2048,
			"createdOn": "2026-01-01 10:00:00",
			"createdBy": "agent.smith",
			"downloadUrl": "https://sn.example.com/download/1"
		}
	}`

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	resp, err := svc.CreateCaseAttachment(contextWithUserIDToken("token"), domain.CreateAttachmentRequest{
		ReferenceID:   sysidToUUID(testWLCaseSysid),
		ReferenceType: domain.ReferenceTypeCase,
		Name:          "diagnostics.log",
		Type:          "text/plain",
		File:          "data:text/plain;base64,ZmFrZQ==",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Attachment.Status != domain.AttachmentStatusComplete {
		t.Fatalf("expected status %q, got %q", domain.AttachmentStatusComplete, resp.Attachment.Status)
	}
	if resp.Attachment.DownloadURL == nil || *resp.Attachment.DownloadURL != "https://sn.example.com/download/1" {
		t.Fatalf("expected a non-nil downloadUrl, got %v", resp.Attachment.DownloadURL)
	}
}
