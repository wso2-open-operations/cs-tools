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

// Package taxonomy is config-driven status-taxonomy helpers shared by
// internal/handler (GET /taxonomy) and internal/metrics (which categorizes
// issues by status without a DB round-trip). Port of v3's
// src/server/lib/taxonomy.ts. Pure, synchronous reads over the in-process
// config — the config itself is already an immutable snapshot loaded once
// at boot, so nothing here needs its own cache.
package taxonomy

import (
	"sort"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
)

// SortedStatusDefs returns every taxonomy row, sortOrder-ascending.
func SortedStatusDefs(cfg *config.AppConfig) []config.StatusEntry {
	statuses := make([]config.StatusEntry, len(cfg.Taxonomy.Statuses))
	copy(statuses, cfg.Taxonomy.Statuses)
	sort.SliceStable(statuses, func(i, j int) bool { return statuses[i].SortOrder < statuses[j].SortOrder })
	return statuses
}

func namesByCategory(cfg *config.AppConfig, category config.StatusCategory) []string {
	names := make([]string, 0)
	for _, s := range SortedStatusDefs(cfg) {
		if s.Category == category {
			names = append(names, s.Name)
		}
	}
	return names
}

// CsStatuses returns status names categorized CS_SIDE (the statuses
// currently on the CS side of the board), sortOrder-ascending.
func CsStatuses(cfg *config.AppConfig) []string {
	return namesByCategory(cfg, config.CategoryCSSide)
}

// ProductSideStatuses returns status names categorized PRODUCT_SIDE (the
// statuses currently owned by the product team), sortOrder-ascending.
func ProductSideStatuses(cfg *config.AppConfig) []string {
	return namesByCategory(cfg, config.CategoryProductSide)
}
