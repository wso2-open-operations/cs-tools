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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
)

// errDashboardTeam marks a generated dashboard whose team filter names no
// usable team: an unknown team, a team in the wrong field (a CRE team in
// sreTeam), a team with no backing group configured, a team filter that is
// not "in", or one inside an anyOf branch. The text says which, readably
// enough to show or hand back to the model.
var errDashboardTeam = errors.New("invalid team filter")

// currentTeamPlaceholder is the "my team" value the webapp resolves itself
// (teamFilterPlaceholder.ts); it passes through untouched.
const currentTeamPlaceholder = "__current_team__"

// dashboardTeam is one registry team a generated dashboard may filter by:
// the filter field its family uses and the group id that field matches on.
type dashboardTeam struct {
	Key     string
	Name    string
	Family  string
	Field   string // "creTeam" or "sreTeam"
	GroupID string // UUID form; "" when the registry configured none
}

// dashboardTeams turns the team a model names in a creTeam / sreTeam filter
// (its registry key, e.g. "rigel", or its name) into the group id the case
// search matches on, once, right after generation. From then on a dashboard
// holds group ids, as the Cases tab's CRE Team filter does, so saving,
// serving and editing need nothing more. A group id already there (a draft
// sent back for a follow-up) is kept when it is a configured team's.
type dashboardTeams struct {
	list      []dashboardTeam
	byName    map[string]dashboardTeam // lower-cased key and name
	byGroupID map[string]dashboardTeam // lower-cased group id
}

// newDashboardTeams builds the lookup from the team registry. CRE families
// (cre-abt, cre) filter on creTeam with the CRE group id, SRE families
// (sre-abt, sre) on sreTeam with the SRE group id; a team of any other family
// (or none) has no case-search team field and is left out.
func newDashboardTeams(results []directory.TeamResult) dashboardTeams {
	t := dashboardTeams{byName: map[string]dashboardTeam{}, byGroupID: map[string]dashboardTeam{}}
	for _, r := range results {
		team := dashboardTeam{Key: r.ID, Name: r.Name, Family: r.Family}
		switch directory.Family(r.Family) {
		case directory.FamilyCREAbt, directory.FamilyCRE:
			team.Field, team.GroupID = "creTeam", strings.ToLower(r.CreGroupID)
		case directory.FamilySREAbt, directory.FamilySRE:
			team.Field, team.GroupID = "sreTeam", strings.ToLower(r.SreGroupID)
		default:
			continue
		}
		t.list = append(t.list, team)
		t.byName[strings.ToLower(team.Key)] = team
		t.byName[strings.ToLower(team.Name)] = team
		if team.GroupID != "" {
			t.byGroupID[team.GroupID] = team
		}
	}
	sort.Slice(t.list, func(i, j int) bool { return t.list[i].Name < t.list[j].Name })
	return t
}

// knownTeams is the readable list of team keys, for an error message.
func (t dashboardTeams) knownTeams() string {
	keys := make([]string, 0, len(t.list))
	for _, team := range t.list {
		keys = append(keys, team.Key)
	}
	return strings.Join(keys, ", ")
}

// resolve swaps every team key or name in the definition's creTeam / sreTeam
// filters (each widget's query and each pie slice's query) for that team's
// group id, and an account query's abtTeam key for the team's name, and checks every team filter: "in" only, top-level filters only
// (not an anyOf branch), a team of the field's family, with a group id
// configured. "__current_team__" is left as it is. Anything else in the
// definition is untouched. A definition that is not a JSON object with a
// widgets array is returned as it is, for ParseGenerated to reject.
func (t dashboardTeams) resolve(definition []byte) ([]byte, error) {
	var d map[string]any
	if err := json.Unmarshal(definition, &d); err != nil {
		return definition, nil
	}
	widgets, ok := d["widgets"].([]any)
	if !ok {
		return definition, nil
	}
	changed := false
	for i, w := range widgets {
		widget, ok := w.(map[string]any)
		if !ok {
			continue
		}
		where := fmt.Sprintf("widgets[%d]", i)
		c, err := t.resolveQuery(widget["query"], where)
		if err != nil {
			return nil, err
		}
		changed = changed || c
		slices, _ := widget["slices"].([]any)
		for j, sl := range slices {
			slice, ok := sl.(map[string]any)
			if !ok {
				continue
			}
			c, err := t.resolveQuery(slice["query"], fmt.Sprintf("%s.slices[%d]", where, j))
			if err != nil {
				return nil, err
			}
			changed = changed || c
		}
	}
	if !changed {
		return definition, nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(d); err != nil {
		return nil, fmt.Errorf("encode dashboard definition: %w", err)
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// resolveQuery handles one query object; changed reports whether a value was swapped.
func (t dashboardTeams) resolveQuery(q any, where string) (changed bool, err error) {
	query, ok := q.(map[string]any)
	if !ok {
		return false, nil
	}
	if v, ok := query["abtTeam"]; ok {
		// An account query's abtTeam: the account search matches the team's
		// name (CRE or SRE team), so a key or name becomes the registry name.
		value, _ := v.(string)
		team, known := t.byName[strings.ToLower(strings.TrimSpace(value))]
		if !known {
			return false, fmt.Errorf("%w: %s: unknown team %q (known teams: %s)", errDashboardTeam, where, value, t.knownTeams())
		}
		if value != team.Name {
			query["abtTeam"] = team.Name
			changed = true
		}
	}
	filters, _ := query["filters"].([]any)
	for _, f := range filters {
		c, err := t.resolveFilter(f, where)
		if err != nil {
			return false, err
		}
		changed = changed || c
	}
	branches, _ := query["anyOf"].([]any)
	for _, b := range branches {
		branch, _ := b.(map[string]any)
		inner, _ := branch["filters"].([]any)
		for _, f := range inner {
			if field := teamFilterField(f); field != "" {
				return false, fmt.Errorf("%w: %s: a %s filter must be in the top-level filters, not an anyOf branch", errDashboardTeam, where, field)
			}
		}
	}
	return changed, nil
}

// teamFilterField is the filter's field when it is creTeam or sreTeam, else "".
func teamFilterField(f any) string {
	filter, _ := f.(map[string]any)
	field, _ := filter["field"].(string)
	if field == "creTeam" || field == "sreTeam" {
		return field
	}
	return ""
}

// resolveFilter swaps the team keys or names in one creTeam / sreTeam filter
// for the teams' group ids, in place, and reports whether anything changed.
// Any other filter is left alone. The filter must use op "in" with at least
// one team; the current-team placeholder passes through.
func (t dashboardTeams) resolveFilter(f any, where string) (bool, error) {
	field := teamFilterField(f)
	if field == "" {
		return false, nil
	}
	filter := f.(map[string]any)
	if op, _ := filter["op"].(string); op != "in" {
		return false, fmt.Errorf("%w: %s: a %s filter must use op \"in\"", errDashboardTeam, where, field)
	}
	values, ok := filter["values"].([]any)
	if !ok || len(values) == 0 {
		return false, fmt.Errorf("%w: %s: a %s filter needs at least one team", errDashboardTeam, where, field)
	}
	changed := false
	for k, v := range values {
		name, _ := v.(string)
		if name == currentTeamPlaceholder {
			continue
		}
		team, err := t.teamFor(name, field, where)
		if err != nil {
			return false, err
		}
		if name != team.GroupID {
			values[k] = team.GroupID
			changed = true
		}
	}
	return changed, nil
}

// teamFor is the team a creTeam / sreTeam value names: a key or name (any
// case), or a group id already resolved; checked against the field.
func (t dashboardTeams) teamFor(value, field, where string) (dashboardTeam, error) {
	team, ok := t.byName[strings.ToLower(strings.TrimSpace(value))]
	if !ok {
		team, ok = t.byGroupID[strings.ToLower(value)]
	}
	if !ok {
		return dashboardTeam{}, fmt.Errorf("%w: %s: unknown team %q (known teams: %s)", errDashboardTeam, where, value, t.knownTeams())
	}
	if team.Field != field {
		return dashboardTeam{}, fmt.Errorf("%w: %s: %s is a %s team: filter it with %s, not %s", errDashboardTeam, where, team.Name, team.Family, team.Field, field)
	}
	if team.GroupID == "" {
		return dashboardTeam{}, fmt.Errorf("%w: %s: team %s has no group configured, so its cases cannot be filtered", errDashboardTeam, where, team.Name)
	}
	return team, nil
}

// resolveDashboard is resolve for a parsed dashboard (dashboard.ParseGenerated's
// result): every widget's and pie slice's query is resolved in place.
func (t dashboardTeams) resolveDashboard(d *dashboard.Dashboard) error {
	for i := range d.Widgets {
		w := &d.Widgets[i]
		where := fmt.Sprintf("widgets[%d]", i)
		if _, err := t.resolveQuery(w.Query, where); err != nil {
			return err
		}
		for j := range w.Slices {
			if _, err := t.resolveQuery(w.Slices[j].Query, fmt.Sprintf("%s.slices[%d]", where, j)); err != nil {
				return err
			}
		}
	}
	return nil
}

// teamResolvingGenerator resolves the team keys in what another generator
// produced, once, right after generation (dashboardTeams). From then on the
// dashboard holds group ids, so the generate reply, Save, My Dashboards and
// Shared Dashboards need nothing more.
type teamResolvingGenerator struct {
	inner DashboardGenerator
	teams dashboardTeams
}

// WithDashboardTeams wraps a generator so that a team named in a creTeam /
// sreTeam filter (its registry key or name) becomes that team's group id, and
// a team filter that cannot work is an errDashboardTeam (the handler's 422,
// with the reason). teams is the registry (directory.TeamResults).
func WithDashboardTeams(inner DashboardGenerator, teams []directory.TeamResult) DashboardGenerator {
	return &teamResolvingGenerator{inner: inner, teams: newDashboardTeams(teams)}
}

// Generate asks the wrapped generator for the dashboard, then resolves its
// team filters (resolveDashboard); an unusable team filter is an error.
func (g *teamResolvingGenerator) Generate(ctx context.Context, prompt string, current json.RawMessage) (GeneratedDashboard, error) {
	result, err := g.inner.Generate(ctx, prompt, current)
	if err != nil {
		return GeneratedDashboard{}, err
	}
	if err := g.teams.resolveDashboard(&result.Dashboard); err != nil {
		return GeneratedDashboard{}, err
	}
	return result, nil
}

// dashboardTeamMessage is an errDashboardTeam's reason as a sentence for the
// chat: the text after the error kind and the widget it names, e.g.
// "Unknown team "orion" (known teams: atlas, castor, ...)."
func dashboardTeamMessage(err error) string {
	parts := strings.SplitN(err.Error(), ": ", 3)
	msg := parts[len(parts)-1]
	if r, size := utf8.DecodeRuneInString(msg); size > 0 {
		msg = string(unicode.ToUpper(r)) + msg[size:]
	}
	if !strings.HasSuffix(msg, ".") {
		msg += "."
	}
	return msg
}
