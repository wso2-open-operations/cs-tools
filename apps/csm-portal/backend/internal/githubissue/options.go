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

// Package githubissue holds the config-driven catalogue of "repository"
// choices the CSM webapp's "Open Git issue" dialog offers a CS engineer,
// resolved once at startup from GITHUB_ISSUE_REPO_OPTIONS (see
// ParseRepoOptions) and served verbatim as the "githubIssueRepoOptions" field
// of GET /metadata (see internal/handler/metadata.go). This mirrors the
// dashboard package's DASHBOARDS_CONFIG shape — a JSON-array env var decoded
// at startup into Go structs and exposed through a read endpoint — without
// that package's directory/hot-reload/preset machinery, which this small,
// single-variable catalogue has no need for.
//
// This existed before as a hardcoded array in the frontend
// (apps/csm-portal/webapp/src/features/csm-cases/components/CreateGithubIssueDialog.tsx),
// which is how a case filed with the wrong "Asgardeo" -> owner/repo mapping
// went to the wrong repository: the mapping lived in code no one reviewing a
// config change would think to check. Moving it into backend config makes
// each option's owner/repo an explicit, reviewable value instead of an
// implicit assumption baked into a dropdown label.
package githubissue

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RepoOption is a single "repository" choice the "Open Git issue" dialog
// offers. Value is an opaque dropdown key (not itself a repo name — Owner and
// Repo carry that), DisplayLabel is the human-readable option text, and
// Owner/Repo are the actual GitHub org/repo an issue filed against this
// option is created in.
//
// GithubLabel is the real GitHub issue label that should eventually be
// applied to an issue filed against this option — distinct from
// DisplayLabel, which is dropdown text only and never sent to GitHub. It is
// stored and served here only: nothing in this backend applies it yet, since
// doing so requires a separate, larger change to the actual issue-filing
// path, which lives outside this backend.
type RepoOption struct {
	Value        string `json:"value"`
	DisplayLabel string `json:"displayLabel"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	GithubLabel  string `json:"githubLabel"`
}

// ParseRepoOptions decodes GITHUB_ISSUE_REPO_OPTIONS, a JSON array of
// RepoOption objects.
//
// An empty value yields no options and no error — a deployment that has not
// configured this yet must still start; the dialog simply has nothing to
// offer. Anything else that fails to parse, or fails validation (a blank
// field, or a duplicate "value" key — the frontend uses Value as the option's
// selection key, so a collision would make two options indistinguishable),
// is an error the caller is expected to make fatal: a silently empty or
// half-populated dropdown is exactly the class of quiet failure that put the
// wrong repo mapping in front of an engineer in the first place.
func ParseRepoOptions(raw string) ([]RepoOption, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var options []RepoOption
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		return nil, fmt.Errorf("GITHUB_ISSUE_REPO_OPTIONS: parse: %w", err)
	}
	// JSON "null" unmarshals into a nil slice with no error, which would
	// otherwise be indistinguishable from an unset env var (the empty-string
	// case above, which is a legitimate "not configured yet" signal). "null"
	// is not: a set-but-null config value should fail loudly, not silently
	// install an empty catalogue. "[]" is fine — json.Unmarshal leaves the
	// slice non-nil for an empty array, so it never trips this check.
	if options == nil {
		return nil, fmt.Errorf("GITHUB_ISSUE_REPO_OPTIONS: must not be JSON null")
	}

	seen := make(map[string]bool, len(options))
	for i, o := range options {
		if strings.TrimSpace(o.Value) == "" {
			return nil, fmt.Errorf("GITHUB_ISSUE_REPO_OPTIONS[%d]: %q is empty", i, "value")
		}
		if seen[o.Value] {
			return nil, fmt.Errorf("GITHUB_ISSUE_REPO_OPTIONS[%d]: duplicate \"value\" %q", i, o.Value)
		}
		seen[o.Value] = true
		if strings.TrimSpace(o.DisplayLabel) == "" {
			return nil, fmt.Errorf("GITHUB_ISSUE_REPO_OPTIONS[%d] (value %q): %q is empty", i, o.Value, "displayLabel")
		}
		if strings.TrimSpace(o.Owner) == "" {
			return nil, fmt.Errorf("GITHUB_ISSUE_REPO_OPTIONS[%d] (value %q): %q is empty", i, o.Value, "owner")
		}
		if strings.TrimSpace(o.Repo) == "" {
			return nil, fmt.Errorf("GITHUB_ISSUE_REPO_OPTIONS[%d] (value %q): %q is empty", i, o.Value, "repo")
		}
		if strings.TrimSpace(o.GithubLabel) == "" {
			return nil, fmt.Errorf("GITHUB_ISSUE_REPO_OPTIONS[%d] (value %q): %q is empty", i, o.Value, "githubLabel")
		}
	}

	return options, nil
}
