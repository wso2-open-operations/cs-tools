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

// Load re-reads the config file from disk on every call rather than caching
// it in a singleton, so each call here reflects only that call's
// SLA_CONFIG_PATH and file state.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `
repos:
  - owner: acme
    name: widgets
    githubProjectId: PVT_test123
    projectTitle: Widgets
    issueQuery: 'label:"Origin/CS"'
taxonomy:
  statuses:
    - { name: "Open", category: PRODUCT_SIDE, accruesSla: true }
budgets:
  - { priority: "Critical(P1)", budgetHours: 24, coverage: "24x7", rank: 1 }
`

// TestLoadHonorsSlaConfigPathOverride verifies Load reads from
// SLA_CONFIG_PATH instead of its default path when the env var is set.
func TestLoadHonorsSlaConfigPathOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(path, []byte(validYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLA_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config to load, got: %v", err)
	}
	if cfg.Repos[0].Owner != "acme" {
		t.Errorf("expected owner=acme, got %q", cfg.Repos[0].Owner)
	}
}

// TestLoadValidatesTheCommittedConfig guards against the repo's actual
// backend/config/sla-config.yaml ever drifting into an invalid state.
func TestLoadValidatesTheCommittedConfig(t *testing.T) {
	t.Setenv("SLA_CONFIG_PATH", "")
	// Load()'s default path is relative to the process cwd, which for `go
	// test` is this package's own directory — point it at the real repo
	// file two levels up instead.
	t.Setenv("SLA_CONFIG_PATH", filepath.Join("..", "..", "config", "sla-config.yaml"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected the committed sla-config.yaml to be valid, got: %v", err)
	}
	// repos is deployment-specific and sensitive: the committed file carries
	// sanitized placeholder entries, while the real orgs/repos are supplied
	// out-of-band at deploy time (e.g. mounted over this file on Choreo).
	// Assert only that at least one is configured, not a specific count.
	if len(cfg.Repos) < 1 {
		t.Errorf("expected at least 1 repo, got %d", len(cfg.Repos))
	}
	if len(cfg.Budgets) != 3 {
		t.Errorf("expected 3 budget tiers, got %d", len(cfg.Budgets))
	}
}

// TestLoadReturnsDescriptiveErrorWhenFileMissing verifies Load's error names
// the missing path rather than surfacing a bare os.Open error.
func TestLoadReturnsDescriptiveErrorWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLA_CONFIG_PATH", filepath.Join(dir, "does-not-exist.yaml"))

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for a missing config file")
	}
	if !strings.Contains(err.Error(), "SLA config not found at") {
		t.Errorf("expected 'SLA config not found at' in error, got: %v", err)
	}
}
