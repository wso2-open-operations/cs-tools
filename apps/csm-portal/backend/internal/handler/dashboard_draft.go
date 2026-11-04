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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// errDraftInvalid marks a draft that is not a dashboard in the served form.
var errDraftInvalid = errors.New("invalid dashboard draft")

// errDraftTooLarge marks a draft whose definition is over
// generateDraftMaxBytes.
var errDraftTooLarge = errors.New("dashboard draft too large")

// generateDraftMaxBytes caps the draft sent with a generate call (64 KiB), the
// size entity-service allows a saved definition: a dashboard that could be
// saved always fits, and it bounds what one call can send the model.
const generateDraftMaxBytes = 64 << 10

// draftToDefinition turns the generate page's draft (the "current" field of
// POST /dashboards/generate) back into the dashboard file format a definition
// is authored in, for the model to edit.
//
// The draft is the dashboard as this backend served it (dashboardDetailView,
// the GET /dashboards/{dashboardId} shape): each widget's id is "widgetId",
// and every case-family query carries the "type" filter load time injected.
// The file format names the id "id" and leaves that filter out (the catalogue
// tells the model it is added automatically), so this renames the one and
// strips the other (dashboard.StripImpliedTypeFilters). Loading the result
// gives back the dashboard the draft was served from.
//
// Only the served fields are read; an unknown field is an error, since a
// draft this backend did not produce is not something to hand the model. An
// absent draft (nil, empty or JSON null) returns nil and no error: the first
// message has none.
func draftToDefinition(current json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(current)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var view dashboardDetailView
	if err := dec.Decode(&view); err != nil {
		return nil, fmt.Errorf("%w: %v", errDraftInvalid, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: data after the dashboard object", errDraftInvalid)
	}

	d := dashboard.Dashboard{
		ID:          view.ID,
		DisplayName: view.DisplayName,
		Type:        view.Type,
		IsDefault:   view.IsDefault,
		TargetTeam:  view.TargetTeam,
		IsTeamBased: view.IsTeamBased,
		Widgets:     widgetTemplates(view.Widgets),
	}
	dashboard.StripImpliedTypeFilters(&d)

	out, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("encode dashboard definition: %w", err)
	}
	return out, nil
}

// widgetTemplates is the reverse of widgetViews: served widgets back to the
// file format's widget templates.
func widgetTemplates(views []dashboardWidgetView) []dashboard.WidgetTemplate {
	templates := make([]dashboard.WidgetTemplate, 0, len(views))
	for _, v := range views {
		var slices []dashboard.PieSlice
		if len(v.Slices) > 0 {
			slices = make([]dashboard.PieSlice, 0, len(v.Slices))
			for _, s := range v.Slices {
				slices = append(slices, dashboard.PieSlice{Label: s.Label, Color: s.Color, Query: s.Query})
			}
		}
		templates = append(templates, dashboard.WidgetTemplate{
			ID:              v.WidgetID,
			DisplayName:     v.DisplayName,
			Description:     v.Description,
			ResourceType:    v.ResourceType,
			Shape:           v.Shape,
			GridWidth:       v.GridWidth,
			Query:           v.Query,
			GroupBy:         v.GroupBy,
			ListLimit:       v.ListLimit,
			Slices:          slices,
			Section:         v.Section,
			Columns:         v.Columns,
			SortBy:          v.SortBy,
			InlineDrilldown: v.InlineDrilldown,
			InlineLabels:    v.InlineLabels,
			Days:            v.Days,
			DistinctBy:      v.DistinctBy,
		})
	}
	return templates
}
