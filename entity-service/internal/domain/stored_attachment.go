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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package domain

import "time"

// StoredAttachmentKind says which attachment table a request is about. These
// are the three whose files live in SFTPGo: work_item_attachment,
// comment_attachment (the images embedded in a comment) and
// deployment_attachment. case_attachment is a separate, older flow.
type StoredAttachmentKind string

const (
	StoredAttachmentWorkItem   StoredAttachmentKind = "work_item"
	StoredAttachmentComment    StoredAttachmentKind = "comment"
	StoredAttachmentDeployment StoredAttachmentKind = "deployment"
)

// Why an attachment has no download link.
const (
	// AttachmentUnavailableNotStored: the row exists but its file is not in
	// SFTPGo (an upload that was never confirmed).
	AttachmentUnavailableNotStored = "NOT_STORED"
	// AttachmentUnavailableFailed: the upload or the copy failed.
	AttachmentUnavailableFailed = "FAILED"
	// AttachmentUnavailableStorage: the file store could not be reached, so no
	// link could be made right now. Trying again later can succeed.
	AttachmentUnavailableStorage = "STORAGE_UNAVAILABLE"
	// AttachmentUnavailableDeleted: the attachment was deleted by the person who
	// uploaded it; its file is gone.
	AttachmentUnavailableDeleted = "DELETED"
)

// AttachmentLinksRequest is the body of POST /attachments/download.
type AttachmentLinksRequest struct {
	Kind StoredAttachmentKind `json:"kind"`
	// IDs are attachment ids of that kind, at most 100.
	IDs []string `json:"ids"`
}

// AttachmentLink is one attachment's metadata and, when its file can be
// fetched, a short-lived link to it. An id the caller may not see is simply
// absent from the response, the same as one that does not exist.
type AttachmentLink struct {
	ID          string  `json:"id"`
	Number      string  `json:"number"`
	Name        string  `json:"name"`
	ContentType *string `json:"contentType,omitempty"`
	SizeBytes   *int64  `json:"sizeBytes,omitempty"`
	Available   bool    `json:"available"`
	// Reason is set when Available is false: one of the AttachmentUnavailable*
	// values.
	Reason string `json:"reason,omitempty"`
	// URL is where the browser fetches the file, valid until ExpiresAt.
	URL       string     `json:"url,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// AttachmentLinksResponse is the body returned by POST /attachments/download.
type AttachmentLinksResponse struct {
	Links []AttachmentLink `json:"links"`
}

// InitiateAttachmentUploadRequest is the body of POST /attachments/upload.
type InitiateAttachmentUploadRequest struct {
	Kind StoredAttachmentKind `json:"kind"`
	// ParentID is the work item the file belongs to (kinds work_item and
	// comment: a comment image is uploaded against the work item before its
	// comment exists) or the deployment (kind deployment).
	ParentID    string `json:"parentId"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// InitiateAttachmentUploadResponse tells the browser where to send the file.
// The browser POSTs a multipart form with the file in the FieldName field to
// UploadURL, exactly once, before ExpiresAt, then calls confirm.
type InitiateAttachmentUploadResponse struct {
	AttachmentID string    `json:"attachmentId"`
	Number       string    `json:"number"`
	UploadURL    string    `json:"uploadUrl"`
	FieldName    string    `json:"fieldName"`
	ExpiresAt    time.Time `json:"expiresAt"`
	MaxBytes     int64     `json:"maxBytes"`
}

// ConfirmAttachmentUploadRequest is the body of
// POST /attachments/upload/{id}/confirm. ID comes from the path.
type ConfirmAttachmentUploadRequest struct {
	Kind StoredAttachmentKind `json:"kind"`
	ID   string               `json:"-"`
}

// ConfirmAttachmentUploadResponse is the stored attachment once its file has
// been checked and moved into place.
type ConfirmAttachmentUploadResponse struct {
	ID          string  `json:"id"`
	Number      string  `json:"number"`
	Name        string  `json:"name"`
	ContentType *string `json:"contentType,omitempty"`
	SizeBytes   int64   `json:"sizeBytes"`
	Status      string  `json:"status"`
}

// DeleteStoredAttachmentRequest is the body of POST /attachments/delete.
type DeleteStoredAttachmentRequest struct {
	// Kind is work_item or deployment; an inline comment image is not deleted
	// on its own.
	Kind StoredAttachmentKind `json:"kind"`
	ID   string               `json:"id"`
}

// StoredAttachmentParent is what an attachment hangs from, as the caller is
// allowed to see it: a work item or a deployment. It carries what the file's
// path is built from and whether new files may still be added.
type StoredAttachmentParent struct {
	ID string
	// Number is the work item's or deployment's own number.
	Number string
	// ProjectNumber is empty when the parent has no project.
	ProjectNumber string
	// Closed is true for a work item that is in state CLOSED. New uploads to it
	// are refused. Always false for a deployment.
	Closed bool
}

// StoredAttachmentRow is one row of an attachment table.
type StoredAttachmentRow struct {
	ID          string
	Number      string
	Name        string
	ContentType *string
	SizeBytes   *int64
	// Status is PENDING, COMPLETE, FAILED, SOURCE_MISSING or DELETED.
	Status     string
	StorageKey *string
	CreatedBy  string
	// ParentID is the work item (kinds work_item and comment) or deployment.
	ParentID string
}

// StoredAttachmentInsert is a new, not yet stored, attachment.
type StoredAttachmentInsert struct {
	ParentID    string
	Name        string
	ContentType string
	SizeBytes   int64
	CreatedBy   string
}
