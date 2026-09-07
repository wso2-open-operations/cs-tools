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

package ingest

import (
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/sla"
)

// RuntimeConfig is the SLA engine wired up from the loaded config file (port
// of v3's src/server/db/sla-config.ts's slaConfigFromFile). Built once at
// boot and shared by ingest, the recompute scheduler, incremental sync, and
// seed.
type RuntimeConfig struct {
	Cfg        sla.Config
	Normalize  StatusNormalizer
	KnownNames map[string]bool
}

// BuildRuntimeConfig derives a RuntimeConfig from app.
func BuildRuntimeConfig(app *config.AppConfig) *RuntimeConfig {
	budgets := make(map[string]float64, len(app.Budgets))
	coverage := make(map[string]sla.Coverage, len(app.Budgets))
	for _, b := range app.Budgets {
		budgets[b.Priority] = b.BudgetHours
		coverage[b.Priority] = sla.Coverage(b.Coverage)
	}

	accrueSet := make(map[string]bool, len(app.Taxonomy.Statuses))
	terminalSet := make(map[string]bool, len(app.Taxonomy.Statuses))
	knownNames := make(map[string]bool, len(app.Taxonomy.Statuses))
	for _, s := range app.Taxonomy.Statuses {
		if s.AccruesSla {
			accrueSet[s.Name] = true
		}
		if s.IsTerminal {
			terminalSet[s.Name] = true
		}
		knownNames[s.Name] = true
	}

	cfg := sla.Config{
		Budgets:  budgets,
		Coverage: coverage,
		Accrues: func(status *string) bool {
			return status != nil && accrueSet[*status]
		},
		IsTerminal: func(status *string) bool {
			return status != nil && terminalSet[*status]
		},
		PossibleThreshold: app.Settings.PossibleThreshold,
	}

	return &RuntimeConfig{
		Cfg:        cfg,
		Normalize:  BuildStatusNormalizer(app.Taxonomy.Aliases),
		KnownNames: knownNames,
	}
}
