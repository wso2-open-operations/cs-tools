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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// attachmentJSONBody builds a well-formed CreateAttachmentRequest/
// CreateCaseAttachmentRequest-compatible JSON body whose total size is
// exactly totalBytes, by padding an unused "padding" field. json.Unmarshal
// into the typed request structs ignores the extra field.
func attachmentJSONBody(totalBytes int) string {
	const prefix = `{"name":"logs.txt","type":"text/plain","content":"aGVsbG8=","padding":"`
	const suffix = `"}`
	padLen := totalBytes - len(prefix) - len(suffix)
	if padLen < 0 {
		padLen = 0
	}
	return prefix + strings.Repeat("x", padLen) + suffix
}

// fakeAttachmentCreateClient records whether entity-service CreateAttachment
// was reached, for asserting the size-limit guard runs before any upstream call.
type fakeAttachmentCreateClient struct {
	entityAttachmentClient
	created bool
}

func (f *fakeAttachmentCreateClient) CreateAttachment(ctx context.Context, req entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error) {
	f.created = true
	return entity.CreateAttachmentResponse{}, nil
}

// TestCreateAttachment_BodySizeLimit covers POST /attachments: a body just
// over the old blanket 1 MiB cap must now succeed (the attachment routes use
// maxAttachmentBodyBytes, 15 MiB), while a body over 15 MiB must still be
// rejected with 413.
func TestCreateAttachment_BodySizeLimit(t *testing.T) {
	tests := map[string]struct {
		bodySize   int
		wantStatus int
	}{
		"just over the old 1 MiB cap succeeds": {
			bodySize:   (1 << 20) + 1024,
			wantStatus: http.StatusCreated,
		},
		"under the 15 MiB attachment cap succeeds": {
			bodySize:   (10 << 20),
			wantStatus: http.StatusCreated,
		},
		"over the 15 MiB attachment cap is rejected": {
			bodySize:   (15 << 20) + 1024,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := &fakeAttachmentCreateClient{}
			h := NewAttachmentHandler(fake)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /attachments", h.CreateAttachment)

			req := authedRequest(http.MethodPost, "/attachments", attachmentJSONBody(tc.bodySize))
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusRequestEntityTooLarge {
				if msg := decodeMessage(t, rec); msg != ErrMsgTooLarge {
					t.Errorf("message = %q, want %q", msg, ErrMsgTooLarge)
				}
				if fake.created {
					t.Error("entity-service CreateAttachment was called for an oversized body")
				}
				return
			}
			if !fake.created {
				t.Error("entity-service CreateAttachment was not called")
			}
		})
	}
}

// TestCreateCaseAttachment_BodySizeLimit covers POST /cases/{id}/attachments
// with the same size boundaries as TestCreateAttachment_BodySizeLimit.
func TestCreateCaseAttachment_BodySizeLimit(t *testing.T) {
	tests := map[string]struct {
		bodySize   int
		wantStatus int
	}{
		"just over the old 1 MiB cap succeeds": {
			bodySize:   (1 << 20) + 1024,
			wantStatus: http.StatusCreated,
		},
		"under the 15 MiB attachment cap succeeds": {
			bodySize:   (10 << 20),
			wantStatus: http.StatusCreated,
		},
		"over the 15 MiB attachment cap is rejected": {
			bodySize:   (15 << 20) + 1024,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := &fakeClosedCaseClient{caseState: "open"}
			h := NewCaseHandler(fake)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /cases/{id}/attachments", h.CreateCaseAttachment)

			req := authedRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments", attachmentJSONBody(tc.bodySize))
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusRequestEntityTooLarge {
				if msg := decodeMessage(t, rec); msg != ErrMsgTooLarge {
					t.Errorf("message = %q, want %q", msg, ErrMsgTooLarge)
				}
				if fake.createAttachment {
					t.Error("entity-service CreateAttachment was called for an oversized body")
				}
				return
			}
			if !fake.createAttachment {
				t.Error("entity-service CreateAttachment was not called")
			}
		})
	}
}
