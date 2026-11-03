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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

// Where attachment files live. It must match entity-service's setting of the
// same name: "sftpgo" keeps the browser-to-SFTPGo links, "backend" has this
// service move the bytes to and from Azure Blob Storage.
const (
	attachmentStorageSFTPGo  = "sftpgo"
	attachmentStorageBackend = "backend"
)

// parseAttachmentStorageMode reads ATTACHMENT_STORAGE_MODE (empty means sftpgo).
func parseAttachmentStorageMode(v string) (string, error) {
	switch m := strings.ToLower(strings.TrimSpace(v)); m {
	case "", attachmentStorageSFTPGo:
		return attachmentStorageSFTPGo, nil
	case attachmentStorageBackend:
		return attachmentStorageBackend, nil
	default:
		return "", fmt.Errorf("ATTACHMENT_STORAGE_MODE must be %q or %q, got %q", attachmentStorageSFTPGo, attachmentStorageBackend, v)
	}
}

// newAttachmentStore builds the Azure store from the environment and makes sure
// its container exists. Sign in as an Entra app (AZURE_TENANT_ID, AZURE_CLIENT_ID,
// AZURE_CLIENT_SECRET with AZURE_STORAGE_ACCOUNT), or with an account key or
// connection string.
func newAttachmentStore(getenv func(string) string) (*azblobstore.Store, error) {
	store, err := azblobstore.New(azblobstore.Config{
		ConnectionString: getenv("AZURE_STORAGE_CONNECTION_STRING"),
		AccountName:      getenv("AZURE_STORAGE_ACCOUNT"),
		AccountKey:       getenv("AZURE_STORAGE_KEY"),
		TenantID:         getenv("AZURE_TENANT_ID"),
		ClientID:         getenv("AZURE_CLIENT_ID"),
		ClientSecret:     getenv("AZURE_CLIENT_SECRET"),
		Endpoint:         getenv("AZURE_STORAGE_ENDPOINT"),
		Container:        getenv("AZURE_STORAGE_CONTAINER"),
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.EnsureContainer(ctx); err != nil {
		return nil, fmt.Errorf("attachment container: %w", err)
	}
	return store, nil
}
