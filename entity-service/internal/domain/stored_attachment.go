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

import (
	"time"
)

// StoredAttachmentKind says which attachment table a request is about. These
// are the three whose files live in Azure Blob Storage: work_item_attachment,
// comment_attachment (the images embedded in a comment) and
// deployment_attachment. case_attachment is an older table that nothing writes or lists any more.
type StoredAttachmentKind string

const (
	StoredAttachmentWorkItem   StoredAttachmentKind = "work_item"
	StoredAttachmentComment    StoredAttachmentKind = "comment"
	StoredAttachmentDeployment StoredAttachmentKind = "deployment"
)

// Why an attachment has no file to download.
const (
	// AttachmentUnavailableNotStored: the row exists but its file is not in
	// storage (an upload that was never confirmed).
	AttachmentUnavailableNotStored = "NOT_STORED"
	// AttachmentUnavailableFailed: the upload or the copy failed.
	AttachmentUnavailableFailed = "FAILED"
	// AttachmentUnavailableStorage: the file store could not be reached right now.
	// Trying again later can succeed.
	AttachmentUnavailableStorage = "STORAGE_UNAVAILABLE"
	// AttachmentUnavailableDeleted: the attachment was deleted by the person who
	// uploaded it; its file is gone.
	AttachmentUnavailableDeleted = "DELETED"
)

// InitiateAttachmentUploadRequest is the body of POST /attachments/upload.
type InitiateAttachmentUploadRequest struct {
	Kind StoredAttachmentKind `json:"type"`
	// ParentID is the work item the file belongs to (kinds work_item and
	// comment: a comment image is uploaded against the work item before its
	// comment exists) or the deployment (kind deployment).
	ParentID string `json:"parentId,omitempty"`
	// ProjectID is for type "comment" only, INSTEAD of ParentID (exactly one of
	// the two): the image is a draft for the description of a record that does not
	// exist yet, uploaded to that project and linked when the record is created.
	ProjectID   string `json:"projectId,omitempty"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// InitiateAttachmentUploadResponse tells the browser how to send the file. The
// portal backend holds the file, so the browser sends it there, in chunks, using
// AttachmentID, then calls confirm. ExpiresAt is when a still-unconfirmed upload
// may be cleaned up.
type InitiateAttachmentUploadResponse struct {
	AttachmentID string    `json:"attachmentId"`
	Number       string    `json:"number"`
	ExpiresAt    time.Time `json:"expiresAt"`
	MaxBytes     int64     `json:"maxBytes"`
	// DirectUpload says the file may be sent straight to Azure through the links of
	// POST /attachments/upload/{id}/direct instead of chunk by chunk through the
	// portal backend. It is only true for a file above the direct-transfer size.
	DirectUpload bool `json:"directUpload"`
}

// DirectUploadLinks is the answer of POST /attachments/upload/{id}/direct: one
// link per chunk. The client sends chunk i (ChunkBytes long, the last one
// shorter) with a PUT to Links[i], then confirms the upload as usual.
type DirectUploadLinks struct {
	Links      []string  `json:"links"`
	ChunkBytes int64     `json:"chunkBytes"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// DirectDownloadLink is the answer of GET /attachments/download/{id}/link: a
// link the client reads the file from, as a download named Name.
type DirectDownloadLink struct {
	URL       string    `json:"url"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ConfirmAttachmentUploadRequest is the body of
// POST /attachments/upload/{id}/confirm. ID comes from the path.
//
// This service cannot look at the file, so the portal backend that committed it
// says what it stored: SizeBytes is
// required, and ContentType is the type it found by reading the file's leading
// bytes (required for a comment image, where it must be a real image type).
type ConfirmAttachmentUploadRequest struct {
	Kind        StoredAttachmentKind `json:"type"`
	ID          string               `json:"-"`
	SizeBytes   *int64               `json:"sizeBytes,omitempty"`
	ContentType string               `json:"contentType,omitempty"`
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

// AuthorizeAttachmentUploadRequest is the body of
// POST /attachments/upload/{id}/authorize. ID comes from the path.
type AuthorizeAttachmentUploadRequest struct {
	Kind StoredAttachmentKind `json:"type"`
	ID   string               `json:"-"`
}

// AttachmentUploadTarget says where the portal backend may write an upload it has
// been authorized to receive. StorageKey is internal: it is for the backend that
// asked and must never reach a browser.
type AttachmentUploadTarget struct {
	AttachmentID string `json:"attachmentId"`
	Number       string `json:"number"`
	Name         string `json:"name"`
	// ContentType is the type declared when the upload started.
	ContentType string `json:"contentType"`
	// SizeBytes is the size declared when the upload started; MaxBytes is the most
	// a file of this kind may be.
	SizeBytes  int64  `json:"sizeBytes"`
	MaxBytes   int64  `json:"maxBytes"`
	StorageKey string `json:"storageKey"`
}

// LocateAttachmentRequest is the body of POST /attachments/locate.
type LocateAttachmentRequest struct {
	Kind StoredAttachmentKind `json:"type"`
	ID   string               `json:"id"`
}

// AttachmentLocation is where an attachment's file is, for the portal backend
// that will read it, once this service has checked the caller may see it.
// StorageKey is internal: it is for the backend that asked and must never reach a
// browser. When the file cannot be read, Available is false and Reason is one of
// the AttachmentUnavailable* values.
type AttachmentLocation struct {
	ID          string  `json:"id"`
	Number      string  `json:"number"`
	Name        string  `json:"name"`
	ContentType *string `json:"contentType,omitempty"`
	SizeBytes   *int64  `json:"sizeBytes,omitempty"`
	Available   bool    `json:"available"`
	Reason      string  `json:"reason,omitempty"`
	StorageKey  string  `json:"storageKey,omitempty"`
	// RelocateTo is set only for an inline image whose file still sits in the
	// drafts folder it was uploaded to (a description image, uploaded before its
	// record existed): the key it belongs under now that it has a work item. The
	// backend copies the file there, removes the old one, then calls
	// POST /attachments/inline/{id}/relocated.
	RelocateTo string `json:"relocateTo,omitempty"`
	// ParentID and Draft are set only on an inline image location. ParentID is the
	// work item the image belongs to, or the project while Draft is true (a
	// description image whose record does not exist yet). The notification service
	// reads the file as an internal caller, which no access rule filters, so it
	// uses these to check the image belongs to the case it is emailing about.
	ParentID string `json:"parentId,omitempty"`
	Draft    bool   `json:"draft,omitempty"`
}

// InlineImageRelocatedRequest is the body of POST /attachments/inline/{id}/relocated.
type InlineImageRelocatedRequest struct {
	// StorageKey is the key the backend moved the file to; it must be the one
	// RelocateTo named.
	StorageKey string `json:"storageKey"`
}

// DeleteStoredAttachmentRequest is the body of POST /attachments/delete.
type DeleteStoredAttachmentRequest struct {
	// Kind is work_item or deployment; an inline comment image is not deleted
	// on its own.
	Kind StoredAttachmentKind `json:"type"`
	ID   string               `json:"id"`
	// AsAdmin is set by the CSM portal backend, and only by it, when the person
	// deleting holds the portal's admin role: they may then delete an attachment
	// they did not upload. Any other caller that sets it is refused, so a browser
	// or the customer portal cannot use it.
	AsAdmin bool `json:"asAdmin,omitempty"`
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
	// ParentID is the work item (kinds work_item and comment) or deployment; for
	// a draft description image it is the project.
	ParentID string
	// Draft is true for a comment image uploaded to a project before the record
	// whose description shows it exists (no work item and no comment yet).
	Draft bool
}

// StoredAttachmentInsert is a new, not yet stored, attachment.
type StoredAttachmentInsert struct {
	ParentID    string
	Name        string
	ContentType string
	SizeBytes   int64
	CreatedBy   string
	// Draft: ParentID is a project, and the row is a draft description image.
	Draft bool
}

// AbandonedUpload is an upload that was started and never confirmed.
type AbandonedUpload struct {
	Kind StoredAttachmentKind
	ID   string
}

// UploadCleanupResult is what one upload cleanup run did.
type UploadCleanupResult struct {
	// Found is how many abandoned uploads were read, Cleaned how many had their
	// staging folder removed and their row marked FAILED, Failed how many could
	// not be (tried again next run).
	Found   int `json:"found"`
	Cleaned int `json:"cleaned"`
	Failed  int `json:"failed"`
	// ImagesRemoved and ImagesFailed are the same for comment images that were
	// uploaded and never linked to a comment.
	ImagesRemoved int `json:"imagesRemoved"`
	ImagesFailed  int `json:"imagesFailed"`
	// ReportDraftsRemoved and ReportDraftsFailed are the same for a security
	// report's files that were uploaded and never linked to a case.
	ReportDraftsRemoved int `json:"reportDraftsRemoved"`
	ReportDraftsFailed  int `json:"reportDraftsFailed"`
	// Orphans are the unlinked comment images and report files whose files the caller must
	// remove from storage. They are NOT yet marked DELETED, so every run lists
	// them again until the caller removes the files and acknowledges them
	// (AcknowledgeOrphansRequest). A lost response therefore loses nothing.
	Orphans []OrphanedImage `json:"orphans,omitempty"`
}

// OrphanedImage is an unlinked comment image or draft report file whose file is
// in storage this service cannot reach. Type says which: "comment" or
// "work_item". StorageKey is internal: it is for the caller that removes the
// file and must never reach a browser.
type OrphanedImage struct {
	ID         string               `json:"id"`
	Type       StoredAttachmentKind `json:"type"`
	StorageKey string               `json:"storageKey"`
}

// AcknowledgeOrphansRequest is the body of POST /attachments/uploads/cleanup/acknowledge:
// the ids of orphaned images whose files the caller has removed from storage.
type AcknowledgeOrphansRequest struct {
	IDs []string `json:"ids"`
}

// AcknowledgeOrphansResult says how many of the acknowledged images were marked
// DELETED, and how many were skipped because they are no longer orphans (already
// acknowledged, or linked to a comment since).
type AcknowledgeOrphansResult struct {
	Acknowledged int `json:"acknowledged"`
	Skipped      int `json:"skipped"`
}

// CompleteAttachmentUploadRequest is the body of POST /attachments/upload/{id}/confirm:
// the number of chunks the browser sent. This service assembles them into the file,
// reads its real size and type, and runs the final checks.
type CompleteAttachmentUploadRequest struct {
	Kind   StoredAttachmentKind `json:"type"`
	ID     string               `json:"-"`
	Chunks int                  `json:"chunks"`
}

// UploadChunkList is the chunks already stored for an upload, so an interrupted
// upload can carry on where it stopped.
type UploadChunkList struct {
	ChunkBytes int           `json:"chunkBytes"`
	Chunks     []UploadChunk `json:"chunks"`
}

// UploadChunk is one stored chunk of an upload.
type UploadChunk struct {
	Index int   `json:"index"`
	Size  int64 `json:"size"`
}
