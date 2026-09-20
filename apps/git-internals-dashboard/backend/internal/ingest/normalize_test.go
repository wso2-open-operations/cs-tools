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

// Tests BuildStatusNormalizer: alias mapping to canonical names, whitespace
// trimming, and nil passthrough.
package ingest

import (
	"testing"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
)

var normalizeAliasesFixture = []config.AliasEntry{
	{Alias: "Re-Opened", Canonical: "Reopened"},
	{Alias: "On-Hold", Canonical: "On Hold"},
}

// strp returns a pointer to s, for building literal *string fixture values.
func strp(s string) *string { return &s }

// TestBuildStatusNormalizerMapsAliasToCanonicalName verifies a configured
// alias normalizes to its canonical status name.
func TestBuildStatusNormalizerMapsAliasToCanonicalName(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(strp("Re-Opened")); got == nil || *got != "Reopened" {
		t.Errorf("expected Reopened, got %v", got)
	}
	if got := normalize(strp("On-Hold")); got == nil || *got != "On Hold" {
		t.Errorf("expected On Hold, got %v", got)
	}
}

// TestBuildStatusNormalizerTrimsWhitespaceAroundNonAliasStatus verifies a
// non-alias status still gets its surrounding whitespace trimmed.
func TestBuildStatusNormalizerTrimsWhitespaceAroundNonAliasStatus(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(strp("  Open  ")); got == nil || *got != "Open" {
		t.Errorf("expected Open, got %v", got)
	}
}

// TestBuildStatusNormalizerPassesNilThrough verifies a nil status stays nil.
func TestBuildStatusNormalizerPassesNilThrough(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(nil); got != nil {
		t.Errorf("expected nil, got %v", *got)
	}
}

// TestBuildStatusNormalizerPassesEmptyStringThroughUnchanged verifies an
// empty (non-nil) status stays the empty string, not nil.
func TestBuildStatusNormalizerPassesEmptyStringThroughUnchanged(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(strp("")); got == nil || *got != "" {
		t.Errorf("expected empty string (not folded into absence), got %v", got)
	}
}

// TestBuildStatusNormalizerPassesUnrecognizedStatusThrough verifies a status
// with no matching alias passes through unchanged.
func TestBuildStatusNormalizerPassesUnrecognizedStatusThrough(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(strp("Open")); got == nil || *got != "Open" {
		t.Errorf("expected Open, got %v", got)
	}
}
