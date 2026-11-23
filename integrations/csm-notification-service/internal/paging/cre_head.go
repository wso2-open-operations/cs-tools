package paging

import (
	"context"
	"strings"
)

// teamMembersReader is the one entity-service read CREHeads needs. Satisfied
// by *EntityClient.
type teamMembersReader interface {
	TeamMembers(ctx context.Context, teamKeys, roles, alertTiers, teamTypes []string) ([]teamMember, error)
}

// CREHeads answers "who is the CRE head" for a caller that only needs an
// address: the SLA breach email, which falls back to the CRE head when a
// case's account has no ABT (or the ABT has no group email), so an at-risk
// or breached SLA is never left with nobody but the assignee told.
//
// It resolves the way the CRE ladder's own CRE head rung does
// (SourceCREHead in TeamScheduleResolver): the configured heads.cre first,
// else everyone holding cre_head in the leadership team. One answer for
// both channels, so a call and an email never name different people.
type CREHeads struct {
	head       Person
	leadership string
	entity     teamMembersReader
}

// NewCREHeads builds the lookup from a CRE ladder configuration. A zero
// LadderConfig (no escalation file, or one that failed to load) still
// works: it falls through to the leadership-team role lookup. entity may be
// nil, in which case only a configured heads.cre can answer.
func NewCREHeads(cfg LadderConfig, entity *EntityClient) CREHeads {
	h := CREHeads{head: cfg.Heads.CRE, leadership: strings.TrimSpace(cfg.Teams.Leadership)}
	if h.leadership == "" {
		h.leadership = defaultLeadershipTeamKey
	}
	if entity != nil {
		h.entity = entity
	}
	return h
}

// CREHeadEmails returns the CRE head's address(es), lower-cased, each once.
// An empty result with a nil error means nobody is configured or holds the
// role; an error means entity-service could not be asked.
//
// A heads.cre with a phone but no email is still the head (the call ladder
// rings it), so it gets no email rather than one sent to the cre_head role
// holders, who may be different people.
func (h CREHeads) CREHeadEmails(ctx context.Context) ([]string, error) {
	if h.head.Set() {
		if email := strings.TrimSpace(h.head.Email); email != "" {
			return []string{strings.ToLower(email)}, nil
		}
		return nil, nil
	}
	if h.entity == nil {
		return nil, nil
	}
	members, err := h.entity.TeamMembers(ctx, []string{h.leadership}, []string{roleCREHead}, nil, nil)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range members {
		email := strings.ToLower(strings.TrimSpace(m.Email))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		out = append(out, email)
	}
	return out, nil
}
