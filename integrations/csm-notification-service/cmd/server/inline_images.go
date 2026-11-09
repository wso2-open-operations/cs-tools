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
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/inlineimages"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

// newInlineImageResolver builds the resolver that puts a comment's or
// description's stored images into notification emails. It is optional:
// without AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_CONTAINER it returns nil and
// a message is logged, and emails simply show no stored image, as before. A
// container that cannot be reached at startup only logs a warning. It only ever reads, so give the app
// the Storage Blob Data Reader role on the one container and nothing more, and
// use an Entra app of its own (AZURE_TENANT_ID, AZURE_CLIENT_ID,
// AZURE_CLIENT_SECRET) rather than the portal backends'.
func newInlineImageResolver(getenv func(string) string, locator inlineimages.Locator) *inlineimages.Resolver {
	account := strings.TrimSpace(getenv("AZURE_STORAGE_ACCOUNT"))
	container := strings.TrimSpace(getenv("AZURE_STORAGE_CONTAINER"))
	if account == "" || container == "" {
		slog.Info("inline images in emails are off: AZURE_STORAGE_ACCOUNT / AZURE_STORAGE_CONTAINER are not set")
		return nil
	}
	endpoint := strings.TrimSpace(getenv("AZURE_STORAGE_ENDPOINT"))
	connectionString, accountKey := getenv("AZURE_STORAGE_CONNECTION_STRING"), getenv("AZURE_STORAGE_KEY")
	// A connection string or account key is shared-key access: full read and
	// write, which is the opposite of the read-only Entra app this feature is
	// meant to run with. They are for Azurite on this machine, so they are used
	// only when the endpoint is a loopback address; anywhere else they are ignored.
	if (connectionString != "" || accountKey != "") && !isLoopbackEndpoint(endpoint) {
		slog.Warn("inline images in emails: AZURE_STORAGE_CONNECTION_STRING / AZURE_STORAGE_KEY ignored, " +
			"they are honoured only with an AZURE_STORAGE_ENDPOINT on localhost")
		connectionString, accountKey = "", ""
	}
	store, err := azblobstore.New(azblobstore.Config{
		AccountName:  account,
		TenantID:     getenv("AZURE_TENANT_ID"),
		ClientID:     getenv("AZURE_CLIENT_ID"),
		ClientSecret: getenv("AZURE_CLIENT_SECRET"),
		Endpoint:     endpoint,
		Container:    container,
		// Development against Azurite only (see above).
		ConnectionString: connectionString,
		AccountKey:       accountKey,
	})
	if err != nil {
		slog.Error("inline images in emails are off: cannot set up the storage client", "err", err)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// A failed check is a warning, not a switch-off: a storage or Entra hiccup
	// while this service starts would otherwise disable the feature until the next
	// restart. Each email already copes with an unreadable image.
	if err := store.CheckContainer(ctx); err != nil {
		slog.Warn("inline images in emails are on, but the storage container was not reachable at startup; "+
			"images are read again for each email", "err", err)
	} else {
		slog.Info("inline images in emails are on")
	}
	return inlineimages.New(locator, store)
}

// isLoopbackEndpoint reports whether a storage endpoint URL points at this machine.
func isLoopbackEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
