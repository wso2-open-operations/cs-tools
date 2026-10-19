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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// SpecialistHandoffConfig is where a specialist handoff ("Escalate to
// specialist team") sends an incident: per product, the services it covers,
// its Special Ops teams and the GitHub repository the internal issue goes
// to. It comes from SPECIALIST_HANDOFF_CONFIG, one line of JSON:
//
//	{"products":[
//	  {"name":"Choreo","serviceIds":["b9c999f8-..."],
//	   "github":{"owner":"wso2-enterprise","repo":"choreo","credential":"wso2-enterprise"},
//	   "teams":[{"key":"choreo-runtime-team","label":"Choreo Runtime Team","groupId":"80dade5d-...",
//	             "smeTeam":"choreo-runtime"}, ...]},
//	  {"name":"U2","serviceIds":["02939644-..."],"teams":[{"key":"u2"}]}]}
//
// A team with no groupId is a Team Schedule team: its key is the team's key,
// and its assignment group is the team itself (ResolveTeamGroups).
//
// ServiceNow hard-codes the same routing in the "Escalate to Special Ops" UI
// action and IncidentHandoffUtils; here a new product or team is a config
// change. A product with one team hands off to it without asking; one with
// several requires the caller to pick one.
type SpecialistHandoffConfig struct {
	Products []SpecialistHandoffProduct `json:"products"`
}

// SpecialistHandoffProduct is one product's routing.
type SpecialistHandoffProduct struct {
	Name       string                        `json:"name"`
	ServiceIDs []string                      `json:"serviceIds"`
	Github     *SpecialistHandoffGithub      `json:"github,omitempty"`
	Teams      []SpecialistHandoffConfigTeam `json:"teams"`
}

// SpecialistHandoffGithub is where a product's internal issue is filed. A
// product without one files no issue. Credential names the token in
// SPECIALIST_HANDOFF_GITHUB_TOKENS that files it; empty means Owner, so one
// token per organisation serves all its repositories, and a repository
// that needs its own token names a credential of its own.
type SpecialistHandoffGithub struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	Credential string `json:"credential,omitempty"`
	// Labels the issue gets, optional: the repositories name their labels
	// differently, so each product maps its own.
	Labels *SpecialistHandoffGithubLabels `json:"labels,omitempty"`
}

// SpecialistHandoffGithubLabels maps what the incident is to the labels its
// issue gets:
//
//	"labels":{"byPriority":{"CRITICAL":["Priority/Highest (P1)","Severity/Critical"],"HIGH":[...]},
//	          "byEnvironment":{"prod":["Env/Production"],"production":["Env/Production"]}}
//
// ByPriority is keyed by the incident priority (CRITICAL, HIGH, MODERATE,
// LOW, PLANNING); ByEnvironment by the incident's environment, compared
// without case. An incident matching no key gets no label from that map.
type SpecialistHandoffGithubLabels struct {
	ByPriority    map[string][]string `json:"byPriority,omitempty"`
	ByEnvironment map[string][]string `json:"byEnvironment,omitempty"`
}

// githubLabelMaxLen is GitHub's limit on a label's name.
const githubLabelMaxLen = 50

// validIncidentPriorities are the keys byPriority accepts.
var validIncidentPriorities = map[string]bool{
	string(domain.IncidentPriorityCritical): true, string(domain.IncidentPriorityHigh): true,
	string(domain.IncidentPriorityModerate): true, string(domain.IncidentPriorityLow): true,
	string(domain.IncidentPriorityPlanning): true,
}

// normalizeLabels trims and checks one map's label lists in place, keying it
// by key(k); where names the map in errors.
func normalizeLabels(m map[string][]string, where string, key func(string) (string, error)) (map[string][]string, error) {
	out := make(map[string][]string, len(m))
	for k, labels := range m {
		nk, err := key(k)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		if _, dup := out[nk]; dup {
			return nil, fmt.Errorf("%s: key %q appears twice", where, k)
		}
		clean := make([]string, 0, len(labels))
		for _, l := range labels {
			l = strings.TrimSpace(l)
			if l == "" || len([]rune(l)) > githubLabelMaxLen {
				return nil, fmt.Errorf("%s[%q]: label %q must be 1 to %d characters", where, k, l, githubLabelMaxLen)
			}
			clean = append(clean, l)
		}
		out[nk] = clean
	}
	return out, nil
}

// labelsFor is the labels an incident's issue gets: its priority's, then its
// environment's, each once, in configured order.
func (l *SpecialistHandoffGithubLabels) labelsFor(priority, environment *string) []string {
	if l == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(labels []string) {
		for _, x := range labels {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	if priority != nil {
		add(l.ByPriority[strings.ToUpper(strings.TrimSpace(*priority))])
	}
	if environment != nil {
		add(l.ByEnvironment[strings.ToLower(strings.TrimSpace(*environment))])
	}
	return out
}

// credential is the name of the token that files this repository's issues.
func (g *SpecialistHandoffGithub) credential() string {
	if g.Credential != "" {
		return g.Credential
	}
	return g.Owner
}

// SpecialistHandoffConfigTeam is one Special Ops team: Key is what the
// handoff names it by (escalationTeam), Label what the dialog shows, GroupID
// the assignment group a handed-off incident and its runbook task go to.
// SMETeam is the Team Schedule team key of the SME rota that answers a
// handoff to it ("asgardeo", "choreo-runtime"): Case Paging pages whoever
// that rota has on duty. Empty means no SME team is named yet; the handoff
// still goes through, and the paging readiness check reports the gap.
//
// A team configured without a groupId is the Team Schedule team keyed Key:
// SMETeam defaults to Key, and ResolveTeamGroups fills GroupID with the
// team's id (and an empty Label with its name) at startup. Until it does --
// no team has the key -- the team takes no handoff.
type SpecialistHandoffConfigTeam struct {
	Key     string `json:"key"`
	Label   string `json:"label,omitempty"`
	GroupID string `json:"groupId,omitempty"`
	SMETeam string `json:"smeTeam,omitempty"`
	// fromTeam marks a team configured without a groupId.
	fromTeam bool
}

// teamKeyPattern is the shape of a Team Schedule team key as the SME rota
// teams are keyed: lower-case letters, digits, '-' and '_', at most 64
// (team.key is VARCHAR(64)). Whether the team exists is not known at startup;
// the paging readiness check says.
var teamKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// teamsForGroup is the product and the teams whose Special Ops group groupID
// is, or nil, nil for any other group. Two teams of one product may share a
// group; the caller then needs the team the handoff picked.
func (c *SpecialistHandoffConfig) teamsForGroup(groupID string) (*SpecialistHandoffProduct, []*SpecialistHandoffConfigTeam) {
	if c == nil || groupID == "" {
		return nil, nil
	}
	for i := range c.Products {
		var teams []*SpecialistHandoffConfigTeam
		for j := range c.Products[i].Teams {
			if g := c.Products[i].Teams[j].GroupID; g != "" && strings.EqualFold(g, groupID) {
				teams = append(teams, &c.Products[i].Teams[j])
			}
		}
		if len(teams) > 0 {
			return &c.Products[i], teams
		}
	}
	return nil, nil
}

// ParseSpecialistHandoffConfig reads and validates SPECIALIST_HANDOFF_CONFIG.
// An empty value is a valid, empty configuration: no service can be handed
// off. Ids are normalised to lower case.
func ParseSpecialistHandoffConfig(raw string) (*SpecialistHandoffConfig, error) {
	cfg := &SpecialistHandoffConfig{}
	if strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("SPECIALIST_HANDOFF_CONFIG: invalid JSON: %w", err)
	}
	seenService := map[string]string{}
	for i := range cfg.Products {
		p := &cfg.Products[i]
		where := fmt.Sprintf("SPECIALIST_HANDOFF_CONFIG products[%d]", i)
		if strings.TrimSpace(p.Name) == "" {
			return nil, fmt.Errorf("%s: name is required", where)
		}
		where += " (" + p.Name + ")"
		if len(p.ServiceIDs) == 0 {
			return nil, fmt.Errorf("%s: serviceIds is required", where)
		}
		for j, id := range p.ServiceIDs {
			id = strings.ToLower(strings.TrimSpace(id))
			if !isCanonicalUUID(id) {
				return nil, fmt.Errorf("%s: serviceIds[%d] %q is not a UUID", where, j, p.ServiceIDs[j])
			}
			if other, dup := seenService[id]; dup {
				return nil, fmt.Errorf("%s: service %s is already in product %s", where, id, other)
			}
			seenService[id] = p.Name
			p.ServiceIDs[j] = id
		}
		if p.Github != nil {
			g := p.Github
			g.Owner, g.Repo, g.Credential = strings.TrimSpace(g.Owner), strings.TrimSpace(g.Repo), strings.TrimSpace(g.Credential)
			if g.Owner == "" || g.Repo == "" {
				return nil, fmt.Errorf("%s: github needs both owner and repo", where)
			}
			if l := g.Labels; l != nil {
				var err error
				if l.ByPriority, err = normalizeLabels(l.ByPriority, where+": github.labels.byPriority", func(k string) (string, error) {
					k = strings.ToUpper(strings.TrimSpace(k))
					if !validIncidentPriorities[k] {
						return "", fmt.Errorf("%q is not an incident priority (CRITICAL, HIGH, MODERATE, LOW, PLANNING)", k)
					}
					return k, nil
				}); err != nil {
					return nil, err
				}
				if l.ByEnvironment, err = normalizeLabels(l.ByEnvironment, where+": github.labels.byEnvironment", func(k string) (string, error) {
					k = strings.ToLower(strings.TrimSpace(k))
					if k == "" {
						return "", fmt.Errorf("an empty environment key")
					}
					return k, nil
				}); err != nil {
					return nil, err
				}
			}
		}
		if len(p.Teams) == 0 {
			return nil, fmt.Errorf("%s: at least one team is required", where)
		}
		seenKey := map[string]bool{}
		for j := range p.Teams {
			t := &p.Teams[j]
			t.Key, t.Label, t.GroupID = strings.TrimSpace(t.Key), strings.TrimSpace(t.Label), strings.ToLower(strings.TrimSpace(t.GroupID))
			t.SMETeam = strings.TrimSpace(t.SMETeam)
			t.fromTeam = t.GroupID == ""
			if t.Key == "" || len(t.Key) > maxEscalationTeamLen || (t.Label == "" && !t.fromTeam) {
				return nil, fmt.Errorf("%s: teams[%d] needs a key (at most %d characters) and a label", where, j, maxEscalationTeamLen)
			}
			if seenKey[t.Key] {
				return nil, fmt.Errorf("%s: team key %q appears twice", where, t.Key)
			}
			seenKey[t.Key] = true
			if t.fromTeam {
				// The team is the Team Schedule team keyed Key, which is
				// then also the SME team it pages.
				if !teamKeyPattern.MatchString(t.Key) {
					return nil, fmt.Errorf("%s: team %q has no groupId, so its key must be a Team Schedule team key (lower-case letters, digits, '-' and '_', at most 64)", where, t.Key)
				}
				if t.SMETeam != "" && t.SMETeam != t.Key {
					return nil, fmt.Errorf("%s: team %q has no groupId, so it is the Team Schedule team %q and its smeTeam cannot be %q", where, t.Key, t.Key, t.SMETeam)
				}
				t.SMETeam = t.Key
			} else if !isCanonicalUUID(t.GroupID) {
				return nil, fmt.Errorf("%s: team %q groupId %q is not a UUID", where, t.Key, t.GroupID)
			}
			if t.SMETeam != "" && !teamKeyPattern.MatchString(t.SMETeam) {
				return nil, fmt.Errorf("%s: team %q smeTeam %q is not a team key (lower-case letters, digits, '-' and '_', at most 64)", where, t.Key, t.SMETeam)
			}
		}
	}
	return cfg, nil
}

// productFor returns the product covering serviceID, or nil.
func (c *SpecialistHandoffConfig) productFor(serviceID *string) *SpecialistHandoffProduct {
	if c == nil || serviceID == nil {
		return nil
	}
	id := strings.ToLower(*serviceID)
	for i := range c.Products {
		for _, s := range c.Products[i].ServiceIDs {
			if s == id {
				return &c.Products[i]
			}
		}
	}
	return nil
}

// team returns the product's team with key, or nil.
func (p *SpecialistHandoffProduct) team(key string) *SpecialistHandoffConfigTeam {
	for i := range p.Teams {
		if p.Teams[i].Key == key {
			return &p.Teams[i]
		}
	}
	return nil
}

// holdsGroup reports whether groupID is one of the product's team groups --
// an incident there is already with Special Ops.
func (p *SpecialistHandoffProduct) holdsGroup(groupID *string) bool {
	if groupID == nil {
		return false
	}
	for _, t := range p.Teams {
		if t.GroupID != "" && strings.EqualFold(t.GroupID, *groupID) {
			return true
		}
	}
	return false
}

// teamOptions is the product's teams as the dialog lists them.
func (p *SpecialistHandoffProduct) teamOptions() []domain.SpecialistHandoffTeam {
	out := make([]domain.SpecialistHandoffTeam, 0, len(p.Teams))
	for _, t := range p.Teams {
		out = append(out, domain.SpecialistHandoffTeam{Key: t.Key, Label: t.Label})
	}
	return out
}

// knowsTeam reports whether any product has a team with key.
func (c *SpecialistHandoffConfig) knowsTeam(key string) bool {
	if c == nil {
		return false
	}
	for i := range c.Products {
		if c.Products[i].team(key) != nil {
			return true
		}
	}
	return false
}

// ResolveTeamGroups gives every team configured without a groupId its
// assignment group: the Team Schedule team keyed by its key, whose id becomes
// GroupID and whose name an empty Label. repo also makes sure "group" has
// that row, which an incident's assignment group must reference. Teams
// already resolved are skipped, so a second call only retries the ones no
// team held. It returns the keys still without a group; those teams take no
// handoff.
//
// Called once at startup, before the configuration is shared: it writes the
// teams in place.
func (c *SpecialistHandoffConfig) ResolveTeamGroups(ctx context.Context, repo repository.SpecialistHandoffGroupRepository) ([]string, error) {
	if c == nil {
		return nil, nil
	}
	var keys []string
	for i := range c.Products {
		for _, t := range c.Products[i].Teams {
			if t.fromTeam && t.GroupID == "" {
				keys = append(keys, t.Key)
			}
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	groups, err := repo.EnsureTeamGroups(ctx, keys)
	if err != nil {
		return keys, err
	}
	var missing []string
	for i := range c.Products {
		for j := range c.Products[i].Teams {
			t := &c.Products[i].Teams[j]
			if !t.fromTeam || t.GroupID != "" {
				continue
			}
			g, ok := groups[strings.ToLower(t.Key)]
			if !ok {
				missing = append(missing, t.Key)
				if t.Label == "" {
					t.Label = t.Key
				}
				continue
			}
			t.GroupID = strings.ToLower(g.ID)
			if t.Label == "" {
				t.Label = g.Name
			}
		}
	}
	return missing, nil
}

// hasUnresolvedTeams reports whether a team configured without a groupId
// still has no group.
func (c *SpecialistHandoffConfig) hasUnresolvedTeams() bool {
	if c == nil {
		return false
	}
	for i := range c.Products {
		for _, t := range c.Products[i].Teams {
			if t.fromTeam && t.GroupID == "" {
				return true
			}
		}
	}
	return false
}

// ResolveSpecialistHandoffTeamGroups runs cfg.ResolveTeamGroups for a
// service starting up, logging what it could not resolve instead of failing:
// the other teams' handoffs, and every other route, still work.
func ResolveSpecialistHandoffTeamGroups(ctx context.Context, repo repository.SpecialistHandoffGroupRepository, cfg *SpecialistHandoffConfig) {
	missing, err := cfg.ResolveTeamGroups(ctx, repo)
	if err != nil {
		slog.ErrorContext(ctx, "specialist handoff: could not read the Team Schedule teams configured without a groupId; they take no handoff until a restart",
			"teams", missing, "error", err)
		return
	}
	if len(missing) > 0 {
		slog.WarnContext(ctx, "specialist handoff: no Team Schedule team has these keys; they take no handoff until the team exists and the service restarts",
			"teams", missing)
	}
}

// Credentials lists the token names the configured products file issues
// with, each once, in configured order.
func (c *SpecialistHandoffConfig) Credentials() []string {
	var out []string
	seen := map[string]bool{}
	if c == nil {
		return out
	}
	for i := range c.Products {
		if g := c.Products[i].Github; g != nil && !seen[g.credential()] {
			seen[g.credential()] = true
			out = append(out, g.credential())
		}
	}
	return out
}

// ParseSpecialistHandoffGithubTokens reads SPECIALIST_HANDOFF_GITHUB_TOKENS:
// a secret, one line of JSON mapping a credential name to a GitHub token,
// {"wso2-enterprise":"github_pat_..."}. Each token needs Issues: write on
// the repositories whose products name it. fallback, when set, serves every
// credential the map does not name. Errors never quote a token.
func ParseSpecialistHandoffGithubTokens(raw, fallback string) (func(credential string) string, error) {
	tokens := map[string]string{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &tokens); err != nil {
			return nil, fmt.Errorf("SPECIALIST_HANDOFF_GITHUB_TOKENS: not a JSON object of credential name to token")
		}
		for name, tok := range tokens {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(tok) == "" {
				return nil, fmt.Errorf("SPECIALIST_HANDOFF_GITHUB_TOKENS: credential %q needs a name and a token", name)
			}
		}
	}
	return func(credential string) string {
		if tok, ok := tokens[credential]; ok {
			return strings.TrimSpace(tok)
		}
		return fallback
	}, nil
}
