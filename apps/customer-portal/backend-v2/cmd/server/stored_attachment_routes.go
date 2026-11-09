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

package main

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/handler"
)

// registerBlobAttachmentRoutes registers the stored-attachment routes. The files
// live in Azure Blob Storage but this service never touches it: entity-service is
// the only service that does, and these handlers forward the bytes to it. The paths
// avoid the case-attachment routes (GET /attachments/{id}/content): see the route test.
func registerBlobAttachmentRoutes(mux *http.ServeMux, stored *handler.StoredAttachmentHandler, blob *handler.BlobAttachmentHandler) {
	mux.HandleFunc("POST /attachments/upload", stored.InitiateUpload)
	mux.HandleFunc("PUT /attachments/upload/{id}/chunks/{index}", blob.PutChunk)
	mux.HandleFunc("GET /attachments/upload/{id}/chunks", blob.ListChunks)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", blob.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", blob.DeleteAttachment)
	mux.HandleFunc("GET /attachments/download/{id}/content", blob.Download)
	mux.HandleFunc("GET /attachments/inline/{id}/content", blob.InlineImage)
}
