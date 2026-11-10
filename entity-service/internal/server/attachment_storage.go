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

package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

const (
	// chunkRequestTimeout bounds one upload chunk: 8 MiB over a slow link. It
	// replaces the ordinary per-request timeout on that route only.
	chunkRequestTimeout = 10 * time.Minute
	// downloadRequestTimeout bounds one file download: 100 MiB over a slow link.
	downloadRequestTimeout = 30 * time.Minute
	// inlineImageRequestTimeout bounds an inline image, which may be moved to its
	// work item's folder as it is read.
	inlineImageRequestTimeout = 5 * time.Minute
)

// newAttachmentFileStore builds the Azure store the attachment files live in, or
// returns nil when none is configured (the file routes then answer 503). A
// container that cannot be reached at startup is logged and not fatal: this
// service serves the whole platform, and a storage outage while it starts must
// not take every other feature down with it. Each transfer reports its own
// failure.
func newAttachmentFileStore(cfg *config.Config) service.FileStore {
	if !cfg.HasAttachmentStorage() {
		slog.Warn("attachment files are off: AZURE_STORAGE_ACCOUNT / AZURE_STORAGE_CONTAINER are not set")
		return nil
	}
	store, err := azblobstore.New(azblobstore.Config{
		ConnectionString: cfg.AzureStorageConnectionString,
		AccountName:      cfg.AzureStorageAccount,
		AccountKey:       cfg.AzureStorageKey,
		TenantID:         cfg.AzureTenantID,
		ClientID:         cfg.AzureClientID,
		ClientSecret:     cfg.AzureClientSecret,
		Endpoint:         cfg.AzureStorageEndpoint,
		Container:        cfg.AzureStorageContainer,
	})
	if err != nil {
		slog.Error("attachment files are off: cannot set up the storage client", "error", err)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Creating a container needs account-level rights that the recommended
	// container-scoped role does not have, so only do it when asked (Azurite and
	// development). Otherwise just check the existing container is reachable.
	if cfg.AzureStorageCreateContainer {
		if err := store.EnsureContainer(ctx); err != nil {
			slog.Error("attachment container could not be created; file transfers will fail until it exists", "error", err)
		}
		return store
	}
	if err := store.CheckContainer(ctx); err != nil {
		slog.Error("attachment container is not reachable; file transfers will fail until it is", "error", err)
	} else {
		slog.Info("attachment files are on")
	}
	return store
}

// directTransferOptions turns on direct transfers (a large file goes between the
// browser and Azure through signed links) when they are asked for and the store
// can sign a link. A link needs the account key, so a store signed in as an Entra
// app cannot; that is logged, and every file keeps going through this service.
func directTransferOptions(cfg *config.Config, store service.FileStore) []service.FileServiceOption {
	if !cfg.AttachmentDirectTransfers {
		return nil
	}
	links, ok := store.(service.LinkStore)
	if !ok || !links.CanSignLinks() {
		slog.Warn("direct attachment transfers are off: they need AZURE_STORAGE_KEY or a connection string with a key, not an Entra app")
		return nil
	}
	slog.Info("direct attachment transfers are on", "aboveBytes", cfg.AttachmentDirectMinBytes)
	return []service.FileServiceOption{service.WithDirectTransfers(links, cfg.AttachmentDirectMinBytes)}
}

// attachmentRequestTimeout gives the routes that move a file the longer limit
// they need; every other route keeps the ordinary REQUEST_TIMEOUT.
func attachmentRequestTimeout(r *http.Request) (time.Duration, bool) {
	p := r.URL.Path
	switch {
	case r.Method == http.MethodPut && strings.HasPrefix(p, "/attachments/upload/") && strings.Contains(p, "/chunks/"):
		return chunkRequestTimeout, true
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/attachments/download/") && strings.HasSuffix(p, "/content"):
		return downloadRequestTimeout, true
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/attachments/inline/") && strings.HasSuffix(p, "/content"):
		return inlineImageRequestTimeout, true
	}
	return 0, false
}
