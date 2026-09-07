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

package handler

import (
	"net/http"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/taxonomy"
)

// TaxonomyHandler serves the config-driven status taxonomy (SPEC §6.2).
// Pure, synchronous reads over the in-process config — no DB round-trip, no
// TTL cache needed (the config itself is already an immutable snapshot
// loaded once at boot).
type TaxonomyHandler struct {
	cfg *config.AppConfig
}

// NewTaxonomyHandler creates a TaxonomyHandler backed by cfg.
func NewTaxonomyHandler(cfg *config.AppConfig) *TaxonomyHandler {
	return &TaxonomyHandler{cfg: cfg}
}

type statusDefWire struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	AccruesSla bool   `json:"accruesSla"`
	IsTerminal bool   `json:"isTerminal"`
	SortOrder  int    `json:"sortOrder"`
}

type taxonomyResponse struct {
	Statuses   []statusDefWire `json:"statuses"`
	CsStatuses []string        `json:"csStatuses"`
}

// GetTaxonomy handles GET /taxonomy.
func (h *TaxonomyHandler) GetTaxonomy(w http.ResponseWriter, r *http.Request) {
	statuses := taxonomy.SortedStatusDefs(h.cfg)
	wire := make([]statusDefWire, len(statuses))
	for i, s := range statuses {
		wire[i] = statusDefWire{
			Name:       s.Name,
			Category:   string(s.Category),
			AccruesSla: s.AccruesSla,
			IsTerminal: s.IsTerminal,
			SortOrder:  s.SortOrder,
		}
	}
	writeJSON(w, http.StatusOK, taxonomyResponse{Statuses: wire, CsStatuses: taxonomy.CsStatuses(h.cfg)})
}
