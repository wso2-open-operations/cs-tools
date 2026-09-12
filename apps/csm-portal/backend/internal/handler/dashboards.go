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
	"sort"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// dashboardPieSliceView is one wedge of a Shape "pie" widget — see
// dashboard.PieSlice. Query is this slice's own criteria verbatim, including
// any unresolved "__current_user__"/"__current_team__" placeholder a filter
// value carries — the caller (frontend) resolves those client-side (see
// apps/csm-portal/webapp/src/features/csm-dashboard/utils/teamFilterPlaceholder.ts
// for the pattern this mirrors), and merges Query under the parent widget's
// own Query itself.
type dashboardPieSliceView struct {
	Label string         `json:"label"`
	Color string         `json:"color,omitempty"`
	Query map[string]any `json:"query"`
}

// dashboardWidgetView is a single widget's filter criteria and display
// metadata, returned as part of GET /dashboards/{dashboardId}. The caller
// resolves each widget's own data by issuing its own POST /{resourceType}s/search
// request (see ResourceType), passing Query as that request's filters.
type dashboardWidgetView struct {
	WidgetID     string                   `json:"widgetId"`
	DisplayName  string                   `json:"displayName"`
	Description  string                   `json:"description,omitempty"`
	ResourceType dashboard.ResourceType   `json:"resourceType"`
	Shape        dashboard.Shape          `json:"shape"`
	GridWidth    int                      `json:"gridWidth"`
	Query        map[string]any           `json:"query"`
	GroupBy      *dashboard.GroupByConfig `json:"groupBy,omitempty"`
	ListLimit    int                      `json:"listLimit,omitempty"`
	Slices       []dashboardPieSliceView  `json:"slices,omitempty"`
	Section      string                   `json:"section,omitempty"`
	// Columns and SortBy are only meaningful for Shape "list" — see
	// dashboard.WidgetTemplate.Columns/SortBy. Forwarded verbatim: Columns
	// is display config the BE never resolves, and SortBy is opaque search
	// criteria like Query, just for that ResourceType's own /search
	// request's "sortBy" instead of its "filters".
	Columns []dashboard.Column `json:"columns,omitempty"`
	SortBy  map[string]any     `json:"sortBy,omitempty"`
}

// filterPresetView is one entry of the shared filter-preset catalogue,
// returned by GET /dashboards/filter-presets. Filter is the single filter
// predicate the preset stands for, verbatim as authored (field/op/values —
// see dashboard.LoadSharedPresets).
//
// Returned as a sorted array rather than a name-keyed object so the order the
// builder's picker shows is deterministic and the shape is expressible in
// OpenAPI without a free-form additionalProperties map.
type filterPresetView struct {
	Name   string         `json:"name"`
	Filter map[string]any `json:"filter"`
}

// sharedSectionView is one entry of the shared reusable-section catalogue,
// returned by GET /dashboards/sections. Widgets is the section's widget run
// in the same shape a dashboard's own widgets are returned in, so the builder
// has one widget model rather than two.
//
// The widgets here are as AUTHORED in the section file: unlike a dashboard's
// widgets, their {"preset": ...} references are NOT expanded and no implied
// "type" filter has been injected, because a section is never loaded through
// a dashboard's finalize pipeline on its own. That is what the builder wants
// — it edits the authored form — but it means these queries are not directly
// usable as search criteria.
type sharedSectionView struct {
	Name        string                `json:"name"`
	DisplayName string                `json:"displayName"`
	Widgets     []dashboardWidgetView `json:"widgets"`
}

// dashboardListItemView is a dashboard's list-level metadata, returned by
// GET /dashboards. IsTeamBased is included here (not just on the detail
// view) so the frontend can decide whether to show a team selector for the
// currently-selected dashboard without waiting on a second fetch.
type dashboardListItemView struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	// Type classifies the dashboard's audience ("cre", "sre", "cs"). The
	// frontend picks the dashboard to open from it plus the caller's own
	// team family, so it has to be on the list view: the choice is made
	// before any detail fetch. Omitted for a definition that predates the
	// field (only possible via the deprecated DASHBOARDS_CONFIG).
	Type        dashboard.Type `json:"type,omitempty"`
	IsDefault   bool           `json:"isDefault"`
	IsTeamBased bool           `json:"isTeamBased"`
	// DefaultForTeamKeys is this dashboard's identity-override list (see
	// dashboard.Dashboard.DefaultForTeamKeys). It has to be on the list
	// view, not just the detail view: the frontend resolves default
	// dashboard selection against the caller's own team key before it ever
	// fetches a dashboard's detail.
	DefaultForTeamKeys []string `json:"defaultForTeamKeys,omitempty"`
}

// dashboardDetailView is a dashboard's full metadata plus its resolved
// widgets, returned by GET /dashboards/{dashboardId}.
type dashboardDetailView struct {
	ID          string                `json:"id"`
	DisplayName string                `json:"displayName"`
	Type        dashboard.Type        `json:"type,omitempty"`
	IsDefault   bool                  `json:"isDefault"`
	TargetTeam  string                `json:"targetTeam"`
	IsTeamBased bool                  `json:"isTeamBased"`
	Widgets     []dashboardWidgetView `json:"widgets"`
}

// DashboardHandler handles HTTP requests for the config-driven dashboard
// widget pilot.
//
// It has no upstream dependency: every widget's Query/Slices are served
// straight from the registry, with any "__current_user__"/"__current_team__"
// placeholder a filter value carries left exactly as configured for the
// frontend to resolve client-side. Resolving the current user's own platform
// id used to require an entity-service round trip (GET /users/me) on every
// request here; that responsibility moved to the frontend, which already
// resolves GET /users/me for its own purposes and can substitute the id
// itself, the same way it already does for "__current_team__" (see
// apps/csm-portal/webapp/src/features/csm-dashboard/utils/teamFilterPlaceholder.ts).
type DashboardHandler struct{}

// NewDashboardHandler creates a DashboardHandler.
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// GetDashboards handles GET /dashboards.
func (h *DashboardHandler) GetDashboards(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	dashboards := dashboard.All()
	views := make([]dashboardListItemView, 0, len(dashboards))
	for _, d := range dashboards {
		views = append(views, dashboardListItemView{
			ID:                 d.ID,
			DisplayName:        d.DisplayName,
			Type:               d.Type,
			IsDefault:          d.IsDefault,
			IsTeamBased:        d.IsTeamBased,
			DefaultForTeamKeys: d.DefaultForTeamKeys,
		})
	}

	writeJSONValue(w, http.StatusOK, views)
}

// GetDashboardDetail handles GET /dashboards/{dashboardId}.
func (h *DashboardHandler) GetDashboardDetail(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	dashboardID := r.PathValue("dashboardId")
	d, ok := dashboard.ByID(dashboardID)
	if !ok {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	widgets := widgetViews(d.Widgets)

	writeJSONValue(w, http.StatusOK, dashboardDetailView{
		ID:          d.ID,
		DisplayName: d.DisplayName,
		Type:        d.Type,
		IsDefault:   d.IsDefault,
		TargetTeam:  d.TargetTeam,
		IsTeamBased: d.IsTeamBased,
		Widgets:     widgets,
	})
}

// widgetViews maps widget templates onto their wire shape. Shared by
// GET /dashboards/{dashboardId} and GET /dashboards/sections so a section's
// widgets and a dashboard's widgets are never two different shapes on the
// wire.
func widgetViews(templates []dashboard.WidgetTemplate) []dashboardWidgetView {
	views := make([]dashboardWidgetView, 0, len(templates))
	for _, tpl := range templates {
		var slices []dashboardPieSliceView
		if len(tpl.Slices) > 0 {
			slices = make([]dashboardPieSliceView, 0, len(tpl.Slices))
			for _, slice := range tpl.Slices {
				slices = append(slices, dashboardPieSliceView{
					Label: slice.Label,
					Color: slice.Color,
					Query: slice.Query,
				})
			}
		}
		views = append(views, dashboardWidgetView{
			WidgetID:     tpl.ID,
			DisplayName:  tpl.DisplayName,
			Description:  tpl.Description,
			ResourceType: tpl.ResourceType,
			Shape:        tpl.Shape,
			GridWidth:    tpl.GridWidth,
			Query:        tpl.Query,
			GroupBy:      tpl.GroupBy,
			ListLimit:    tpl.ListLimit,
			Slices:       slices,
			Section:      tpl.Section,
			Columns:      tpl.Columns,
			SortBy:       tpl.SortBy,
		})
	}
	return views
}

// GetFilterPresets handles GET /dashboards/filter-presets.
//
// Lists the shared filter presets a dashboard definition may reference by
// name. This is builder-support, not dashboard-serving: a served dashboard
// has every preset reference already expanded.
//
// A deployment with no presets file configured is legal and returns an empty
// array, not an error — the builder then simply offers no presets.
func (h *DashboardHandler) GetFilterPresets(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	presets := dashboard.FilterPresets()
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)

	views := make([]filterPresetView, 0, len(names))
	for _, name := range names {
		views = append(views, filterPresetView{Name: name, Filter: presets[name]})
	}

	writeJSONValue(w, http.StatusOK, views)
}

// GetSharedSections handles GET /dashboards/sections.
//
// Lists the shared reusable sections a dashboard definition may pull in by
// name via "includeSections". Same builder-support contract, and same
// empty-array-not-error behaviour, as GetFilterPresets.
func (h *DashboardHandler) GetSharedSections(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	sections := dashboard.SharedSections()
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names)

	views := make([]sharedSectionView, 0, len(names))
	for _, name := range names {
		s := sections[name]
		views = append(views, sharedSectionView{
			Name:        name,
			DisplayName: s.DisplayName,
			Widgets:     widgetViews(s.Widgets),
		})
	}

	writeJSONValue(w, http.StatusOK, views)
}
