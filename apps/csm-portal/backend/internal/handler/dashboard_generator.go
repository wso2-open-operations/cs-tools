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
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// DashboardGenerator turns one chat message into a dashboard for
// POST /dashboards/generate.
//
// prompt is already trimmed and length-checked by the handler. current is the
// dashboard being edited, as the webapp sent it (GET /dashboards/{id}
// format), or nil for the first message. The result's Dashboard is already
// parsed and checked by the generator itself, so the handler only converts it
// to the wire format. Implementations: sampleDashboardGenerator (today), a
// Claude-backed one (later).
type DashboardGenerator interface {
	Generate(ctx context.Context, prompt string, current json.RawMessage) (GeneratedDashboard, error)
}

// GeneratedDashboard is one generator turn: the dashboard to preview and the
// chat reply to show.
type GeneratedDashboard struct {
	Dashboard dashboard.Dashboard
	Reply     string
}

// ErrGenerateUnavailable means the generator cannot answer at all right now
// (the handler's 503), as opposed to a failed attempt.
var ErrGenerateUnavailable = errors.New("dashboard generation unavailable")

// generateSampleDashboardID is the dashboard the sample generator returns: a
// custom dashboard already loaded into the registry from DASHBOARDS_DIR
// (01.json in dashboards.example, "My Cases and Escalations").
const generateSampleDashboardID = "5e636a12-4c0c-4d86-9ee5-6f4e0eddf981"

// sampleDashboardGenerator answers every message with
// generateSampleDashboardID from the registry, ignoring the prompt and the
// draft. It is the fallback when no AI integration service is configured.
type sampleDashboardGenerator struct{}

// NewSampleDashboardGenerator returns the sample generator.
func NewSampleDashboardGenerator() DashboardGenerator {
	return sampleDashboardGenerator{}
}

// Generate returns the sample, or ErrGenerateUnavailable when it is not loaded.
func (sampleDashboardGenerator) Generate(_ context.Context, _ string, _ json.RawMessage) (GeneratedDashboard, error) {
	d, ok := dashboard.ByID(generateSampleDashboardID)
	if !ok {
		return GeneratedDashboard{}, fmt.Errorf("sample dashboard %q not loaded: %w", generateSampleDashboardID, ErrGenerateUnavailable)
	}
	count := len(d.Widgets)
	plural := "s"
	if count == 1 {
		plural = ""
	}
	return GeneratedDashboard{
		Dashboard: d,
		Reply: fmt.Sprintf(
			"Here's a sample dashboard for now: %s, %d widget%s. Generating from your description comes in a later step.",
			d.DisplayName, count, plural),
	}, nil
}
