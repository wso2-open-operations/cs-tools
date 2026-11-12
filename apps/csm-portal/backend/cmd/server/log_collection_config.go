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
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
)

var catalogItemIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var githubNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// loadLogCollectionConfig returns enabled=false when LOG_COLLECTION_CATALOG_ITEM_ID
// is unset. Otherwise LOG_COLLECTION_REPO ("owner/repo") is required too.
func loadLogCollectionConfig(getenv func(string) string) (cfg handler.LogCollectionConfig, enabled bool, err error) {
	itemID := strings.TrimSpace(getenv("LOG_COLLECTION_CATALOG_ITEM_ID"))
	if itemID == "" {
		return handler.LogCollectionConfig{}, false, nil
	}
	if !catalogItemIDPattern.MatchString(itemID) {
		return handler.LogCollectionConfig{}, false, errors.New("LOG_COLLECTION_CATALOG_ITEM_ID is not a UUID")
	}
	ownerAndRepo := strings.TrimSpace(getenv("LOG_COLLECTION_REPO"))
	owner, repo, hasSlash := strings.Cut(ownerAndRepo, "/")
	if !hasSlash || !githubNamePattern.MatchString(owner) || !githubNamePattern.MatchString(repo) {
		return handler.LogCollectionConfig{}, false, fmt.Errorf("LOG_COLLECTION_REPO must be owner/repo, got %q", ownerAndRepo)
	}
	return handler.LogCollectionConfig{
		CatalogItemID: itemID,
		Owner:         owner,
		Repo:          repo,
		Team:          strings.TrimSpace(getenv("LOG_COLLECTION_TEAM")),
	}, true, nil
}
