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

// Port of v3's src/server/db/normalize.test.ts — every case there has a
// direct counterpart here.
package ingest

import (
	"testing"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
)

var normalizeAliasesFixture = []config.AliasEntry{
	{Alias: "Re-Opened", Canonical: "Reopened"},
	{Alias: "On-Hold", Canonical: "On Hold"},
}

func strp(s string) *string { return &s }

func TestBuildStatusNormalizerMapsAliasToCanonicalName(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(strp("Re-Opened")); got == nil || *got != "Reopened" {
		t.Errorf("expected Reopened, got %v", got)
	}
	if got := normalize(strp("On-Hold")); got == nil || *got != "On Hold" {
		t.Errorf("expected On Hold, got %v", got)
	}
}

func TestBuildStatusNormalizerTrimsWhitespaceAroundNonAliasStatus(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(strp("  Open  ")); got == nil || *got != "Open" {
		t.Errorf("expected Open, got %v", got)
	}
}

func TestBuildStatusNormalizerPassesNilThrough(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(nil); got != nil {
		t.Errorf("expected nil, got %v", *got)
	}
}

func TestBuildStatusNormalizerPassesEmptyStringThroughUnchanged(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(strp("")); got == nil || *got != "" {
		t.Errorf("expected empty string (not folded into absence), got %v", got)
	}
}

func TestBuildStatusNormalizerPassesUnrecognizedStatusThrough(t *testing.T) {
	normalize := BuildStatusNormalizer(normalizeAliasesFixture)
	if got := normalize(strp("Open")); got == nil || *got != "Open" {
		t.Errorf("expected Open, got %v", got)
	}
}
