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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SpecialistHandoffConfig is where a specialist handoff ("Escalate to
// specialist team") sends an incident: per product, the services it covers,
// its Special Ops teams and the GitHub repository the internal issue goes
// to. It comes from SPECIALIST_HANDOFF_CONFIG, one line of JSON:
//
//	{"products":[
//	  {"name":"Choreo","serviceIds":["b9c999f8-..."],
//	   "github":{"owner":"wso2-enterprise","repo":"choreo","credential":"wso2-enterprise"},
//	   "teams":[{"key":"choreo-runtime-team","label":"Choreo Runtime Team","groupId":"80dade5d-..."}, ...]},
//	  {"name":"Asgardeo", ...}]}
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
type SpecialistHandoffConfigTeam struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	GroupID string `json:"groupId"`
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
		}
		if len(p.Teams) == 0 {
			return nil, fmt.Errorf("%s: at least one team is required", where)
		}
		seenKey := map[string]bool{}
		for j := range p.Teams {
			t := &p.Teams[j]
			t.Key, t.Label, t.GroupID = strings.TrimSpace(t.Key), strings.TrimSpace(t.Label), strings.ToLower(strings.TrimSpace(t.GroupID))
			if t.Key == "" || len(t.Key) > maxEscalationTeamLen || t.Label == "" {
				return nil, fmt.Errorf("%s: teams[%d] needs a key (at most %d characters) and a label", where, j, maxEscalationTeamLen)
			}
			if seenKey[t.Key] {
				return nil, fmt.Errorf("%s: team key %q appears twice", where, t.Key)
			}
			seenKey[t.Key] = true
			if !isCanonicalUUID(t.GroupID) {
				return nil, fmt.Errorf("%s: team %q groupId %q is not a UUID", where, t.Key, t.GroupID)
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
		if strings.EqualFold(t.GroupID, *groupID) {
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
