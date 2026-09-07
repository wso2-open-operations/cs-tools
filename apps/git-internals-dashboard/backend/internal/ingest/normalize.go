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

// Package ingest is the transport-agnostic write path for GitHub-derived
// data (SPEC §8.5): the seed, the incremental sync, and any future webhook
// handler all normalize through it, so it must not know or care which
// transport produced its input.
package ingest

import (
	"strings"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
)

// StatusNormalizer maps a raw board status string to its canonical taxonomy
// name (port of normalize.ts's buildStatusNormalizer). One implementation
// shared by the seed, the recompute job, and the incremental sync.
type StatusNormalizer func(status *string) *string

// BuildStatusNormalizer builds a StatusNormalizer from the taxonomy's alias
// list. A status is trimmed, then looked up in the alias map exactly —
// whitespace-only folding is safe, but anything more (case, hyphens) must be
// an explicit alias row, since automatic folding could merge intentionally
// distinct statuses.
func BuildStatusNormalizer(aliases []config.AliasEntry) StatusNormalizer {
	m := make(map[string]string, len(aliases))
	for _, a := range aliases {
		m[a.Alias] = a.Canonical
	}
	return func(status *string) *string {
		if status == nil {
			return nil
		}
		trimmed := strings.TrimSpace(*status)
		if canonical, ok := m[trimmed]; ok {
			return &canonical
		}
		return &trimmed
	}
}
