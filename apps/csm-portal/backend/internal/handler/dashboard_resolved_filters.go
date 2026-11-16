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
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	plgdomain "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// maxResolvedFilters caps the filters one POST /dashboards/resolved-filters
// call takes: a code dashboard's case call has a handful.
const maxResolvedFilters = 50

// maxResolvedTeamIDs caps a time card team list, as the time card search's
// teamIds does.
const maxResolvedTeamIDs = 100

// resolvedFiltersBody is the body and the reply of
// POST /dashboards/resolved-filters: a case search's filter list, a time
// card search's team list, a PLG call's playbook task and answers, or any
// of them together. What was not sent is not returned; the playbook codes
// are only checked, and come back as they were sent.
type resolvedFiltersBody struct {
	Filters         []any  `json:"filters,omitempty"`
	TeamIDs         []any  `json:"teamIds,omitempty"`
	PlaybookTask    string `json:"playbookTask,omitempty"`
	PlaybookAnswers []any  `json:"playbookAnswers,omitempty"`
}

// DashboardResolvedFiltersHandler serves POST /dashboards/resolved-filters
// for the code approach (off unless DASHBOARD_GENERATE_CODE_ENABLED is
// "true"). A code dashboard's case filters name products, versions, projects,
// customers, CRE/SRE teams and regions as the user wrote them, and its time
// card calls name teams (teamIds); the JSON approach turns those into what
// the case search takes right after generation (the
// WithDashboard* generators), but code has no such step, so the webapp's
// portal bridge sends the filters here when a call runs. The same resolvers
// and rules apply, so a name matching nothing or several records is the same
// 422 with the same reason; a failed lookup is a 502. It reads only, and only
// what the caller's own searches return.
type DashboardResolvedFiltersHandler struct {
	products  productSearcher
	projects  namedKind
	accounts  namedKind
	teams     dashboardTeams
	playbooks func(context.Context) ([]plgdomain.Playbook, error)
	cases     caseAggregator
}

// WithPlaybooks makes the handler check a PLG call's playbook task and
// answer codes against the PLG playbooks' answer fields, read with list on
// every check (an admin may have edited one), as the generate path does
// (checkPlaybookQuery). Without it, or when they cannot be read, no task is
// known, as for a generated dashboard.
func (h *DashboardResolvedFiltersHandler) WithPlaybooks(list func(context.Context) ([]plgdomain.Playbook, error)) *DashboardResolvedFiltersHandler {
	h.playbooks = list
	return h
}

// WithRegions makes the handler check a case filter's regions against the
// regions of the cases the caller can see and write each as the account
// record does, as the generate path does (WithDashboardRegions). Without it,
// a region filter passes through as it was sent.
func (h *DashboardResolvedFiltersHandler) WithRegions(cases caseAggregator) *DashboardResolvedFiltersHandler {
	h.cases = cases
	return h
}

// NewDashboardResolvedFiltersHandler returns the handler, searching products,
// projects and accounts through the entity service and teams in the registry.
func NewDashboardResolvedFiltersHandler(products productSearcher, projects projectSearcher, accounts accountSearcher, teams []directory.TeamResult) *DashboardResolvedFiltersHandler {
	return &DashboardResolvedFiltersHandler{
		products: products,
		projects: projectKind(projects),
		accounts: accountKind(accounts),
		teams:    newDashboardTeams(teams),
	}
}

// ResolveFilters handles POST /dashboards/resolved-filters.
func (h *DashboardResolvedFiltersHandler) ResolveFilters(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	var body resolvedFiltersBody
	if !decodeJSONBody(w, r, &body) {
		return
	}
	msg := ""
	switch {
	case body.Filters == nil && body.TeamIDs == nil && body.PlaybookTask == "" && body.PlaybookAnswers == nil:
		msg = "filters, teamIds or playbookTask is required"
	case body.PlaybookAnswers != nil && body.PlaybookTask == "":
		msg = "playbookAnswers needs a playbookTask"
	case body.Filters != nil:
		msg = checkResolvedFilters(body.Filters)
	}
	if msg == "" && body.TeamIDs != nil {
		msg = checkResolvedTeamIDs(body.TeamIDs)
	}
	if msg == "" {
		msg = checkResolvedPlaybookAnswers(body.PlaybookAnswers)
	}
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.checkPlaybook(r.Context(), body); err != nil {
		writeResolvedFiltersError(w, r, err)
		return
	}
	if err := h.resolve(r, body); err != nil {
		writeResolvedFiltersError(w, r, err)
		return
	}
	writeJSONValue(w, http.StatusOK, body)
}

// checkPlaybook is why body's playbook task or answers cannot be used (nil
// when they can, or when there are none): the generate path's own check, on
// the playbooks as they are now.
func (h *DashboardResolvedFiltersHandler) checkPlaybook(ctx context.Context, body resolvedFiltersBody) error {
	if body.PlaybookTask == "" {
		return nil
	}
	var playbooks []plgdomain.Playbook
	if h.playbooks != nil {
		var err error
		if playbooks, err = h.playbooks(ctx); err != nil {
			slog.WarnContext(ctx, "dashboard resolved filters: PLG playbooks unavailable; no playbook task is known", "err", err)
		}
	}
	fields := playbookAnswerFields(playbooks)
	byCode := make(map[string]csmai.PlaybookAnswerField, len(fields))
	for _, f := range fields {
		byCode[f.Code] = f
	}
	query := map[string]any{"playbookTask": body.PlaybookTask}
	if body.PlaybookAnswers != nil {
		query["playbookAnswers"] = body.PlaybookAnswers
	}
	return checkPlaybookQuery("playbook", query, nil, byCode, fields)
}

// resolve rewrites body's lists in place: teams (the filters' and a time
// card team list), then products and their versions, then projects, then
// accounts, then regions, as the generators do.
func (h *DashboardResolvedFiltersHandler) resolve(r *http.Request, body resolvedFiltersBody) error {
	ctx := r.Context()
	const where = "filters"
	query := map[string]any{}
	if body.TeamIDs != nil {
		query["teamIds"] = body.TeamIDs
	}
	if body.Filters != nil {
		query["filters"] = body.Filters
	}
	if _, err := h.teams.resolveQuery(query, where, true); err != nil {
		return err
	}
	if body.Filters == nil {
		return nil
	}
	products := productResolver{search: h.products, names: map[string]string{}, ids: map[string][]string{}, versions: map[string][]string{}}
	if err := products.resolveQuery(ctx, query, where); err != nil {
		return err
	}
	if err := products.resolveVersions(ctx, query, queryProduct(query), where); err != nil {
		return err
	}
	if err := (namedResolver{kind: h.projects, ids: map[string]string{}}).resolveQuery(ctx, query, where); err != nil {
		return err
	}
	if err := (namedResolver{kind: h.accounts, ids: map[string]string{}}).resolveQuery(ctx, query, where); err != nil {
		return err
	}
	if h.cases == nil {
		return nil
	}
	return (&regionResolver{cases: h.cases}).resolveQuery(ctx, query, where)
}

// checkResolvedFilters is why filters cannot be resolved ("" when they can):
// at most maxResolvedFilters, each an object with a field and an op; a named
// field's values are one or more strings, its op one the generators accept;
// a version sits beside exactly one product. Every other filter is the case
// search's to check, and passes through untouched.
func checkResolvedFilters(filters []any) string {
	if filters == nil {
		return "filters is required"
	}
	if len(filters) > maxResolvedFilters {
		return fmt.Sprintf("at most %d filters", maxResolvedFilters)
	}
	versions, products := 0, map[string]bool{}
	for i, f := range filters {
		entry, ok := f.(map[string]any)
		if !ok {
			return fmt.Sprintf("filters[%d] is not an object", i)
		}
		field, _ := entry["field"].(string)
		op, _ := entry["op"].(string)
		if field == "" || op == "" {
			return fmt.Sprintf("filters[%d] needs a field and an op", i)
		}
		ops, named := resolvedFilterOps[field]
		if !named {
			continue
		}
		if !ops[op] {
			return fmt.Sprintf("filters[%d]: a %s filter does not take op %q", i, field, op)
		}
		values, _ := entry["values"].([]any)
		if len(values) == 0 {
			return fmt.Sprintf("filters[%d]: a %s filter needs at least one value", i, field)
		}
		for _, v := range values {
			if _, ok := v.(string); !ok {
				return fmt.Sprintf("filters[%d]: a %s filter's values are strings", i, field)
			}
		}
		switch {
		case field == "productVersion":
			versions++
		case field == "product" && op == "in":
			for _, v := range values {
				products[v.(string)] = true
			}
		}
	}
	if versions > 0 && len(products) != 1 {
		return "a productVersion filter needs a product filter (in) naming exactly one product"
	}
	return ""
}

// checkResolvedTeamIDs is why a time card team list cannot be resolved (""
// when it can): 1 to maxResolvedTeamIDs team keys or names. "My team" is
// not one: a code dashboard has no team picker to read it from.
func checkResolvedTeamIDs(teamIDs []any) string {
	if teamIDs == nil {
		return ""
	}
	if len(teamIDs) == 0 || len(teamIDs) > maxResolvedTeamIDs {
		return fmt.Sprintf("teamIds needs 1 to %d teams", maxResolvedTeamIDs)
	}
	for i, v := range teamIDs {
		name, ok := v.(string)
		if !ok || name == "" {
			return fmt.Sprintf("teamIds[%d] is not a team key or name", i)
		}
		if name == currentTeamPlaceholder {
			return fmt.Sprintf("teamIds[%d]: %s is not resolved here", i, currentTeamPlaceholder)
		}
	}
	return ""
}

// checkResolvedPlaybookAnswers is why playbook answers cannot be checked (""
// when they can): 1 to maxPlaybookAnswerOptions non-empty codes.
func checkResolvedPlaybookAnswers(answers []any) string {
	if answers == nil {
		return ""
	}
	if len(answers) == 0 || len(answers) > maxPlaybookAnswerOptions {
		return fmt.Sprintf("playbookAnswers needs 1 to %d answer codes", maxPlaybookAnswerOptions)
	}
	for i, v := range answers {
		if code, ok := v.(string); !ok || code == "" {
			return fmt.Sprintf("playbookAnswers[%d] is not an answer code", i)
		}
	}
	return ""
}

// resolvedFilterOps is each named field and the ops the generators accept
// for it.
var resolvedFilterOps = map[string]map[string]bool{
	"product":        {"in": true, "notIn": true},
	"productVersion": {"in": true},
	"project":        {"in": true, "notIn": true},
	"account":        {"in": true, "notIn": true},
	"creTeam":        {"in": true},
	"sreTeam":        {"in": true},
	"region":         {"in": true, "notIn": true},
}

// writeResolvedFiltersError writes a resolver's error as the generate
// handler does: a name that cannot be used is a 422 with the reason (it may
// name a customer, so the log does not hold it), a failed lookup a 502.
func writeResolvedFiltersError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	for _, c := range []struct {
		unusable, lookup error
		code, lookupCode string
		lookupMsg        string
	}{
		{errDashboardTeam, nil, errCodeGenerateTeam, "", ""},
		{errDashboardPlaybook, nil, errCodeGeneratePlaybook, "", ""},
		{errDashboardProduct, errDashboardProductLookup, errCodeGenerateProduct, errCodeGenerateProductLookup, errMsgGenerateProductLookup},
		{errDashboardProject, errDashboardProjectLookup, errCodeGenerateProject, errCodeGenerateProjectLookup, errMsgGenerateProjectLookup},
		{errDashboardAccount, errDashboardAccountLookup, errCodeGenerateAccount, errCodeGenerateAccountLookup, errMsgGenerateAccountLookup},
		{errDashboardRegion, errDashboardRegionLookup, errCodeGenerateRegion, errCodeGenerateRegionLookup, errMsgGenerateRegionLookup},
	} {
		if errors.Is(err, c.unusable) {
			slog.InfoContext(ctx, "dashboard resolved filters: unusable name", "code", c.code)
			writeErrorCode(w, http.StatusUnprocessableEntity, dashboardTeamMessage(err), c.code)
			return
		}
		if c.lookup != nil && errors.Is(err, c.lookup) {
			slog.WarnContext(ctx, "dashboard resolved filters: lookup failed", "code", c.lookupCode, "err", err)
			writeErrorCode(w, http.StatusBadGateway, c.lookupMsg, c.lookupCode)
			return
		}
	}
	slog.ErrorContext(ctx, "dashboard resolved filters: failed", "err", err)
	writeError(w, http.StatusInternalServerError, ErrMsgInternal)
}
