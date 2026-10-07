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

// Package directory holds the portal's reference catalogues -- the team
// registry and the assignable-role allow-list -- as deployment configuration
// resolved once at startup and served from memory thereafter.
//
// Both were previously configuration of the entity service, which meant every
// team-catalogue request and every team-name lookup crossed a service boundary
// to read a value that never changes while the process runs. They live here
// now: the mapping between a team's key, its display name in the backing data
// source, that group's id, and this platform's UUID form of that id is derivable
// from the configured rows alone, with no upstream call at all. Resolving it
// once at startup makes the whole catalogue a memory read.
//
// What did NOT move, because it genuinely cannot: membership. Which users
// belong to which group is live state, so /users/me team resolution and the
// user-search team filter still go upstream -- but they go upstream carrying
// group names this package resolved, rather than asking the entity service to
// consult a registry it no longer has.
package directory

import (
	"fmt"
	"strings"
)

// Family classifies a team along two axes at once: its discipline (customer
// renewal/expansion vs site reliability) and whether it is an account-based
// team (ABT) or the wider non-ABT organisation. Not every team has a family
// assigned -- it is empty for the unclassified teams.
//
// The "-abt" variants are the ones a dashboard team picker offers: a dashboard
// scoped to a discipline lists only that discipline's ABTs. The bare variants
// classify a member of the discipline who is not on an account-based team.
type Family string

const (
	// FamilyCREAbt identifies a Customer Renewal & Expansion account-based
	// team.
	FamilyCREAbt Family = "cre-abt"
	// FamilyCRE identifies a Customer Renewal & Expansion team that is not an
	// account-based team.
	FamilyCRE Family = "cre"
	// FamilySREAbt identifies a Site Reliability Engineering account-based
	// team.
	FamilySREAbt Family = "sre-abt"
	// FamilySRE identifies a Site Reliability Engineering team that is not an
	// account-based team.
	FamilySRE Family = "sre"
	// FamilySME identifies a team on one of the SME (special) product
	// rotations -- Asgardeo, Choreo Runtime, Moesif and the like. It has no
	// dashboard type of its own, so a member lands on the CS-wide default, as
	// a caller with no team does.
	FamilySME Family = "sme"
)

// validFamilies is the closed set of family values the registry accepts.
//
// It is closed deliberately: consumers branch on the family (a dashboard's team
// picker filters to "sre-abt", the default-dashboard choice keys off
// "cre"/"sre"), so a typo like "sre_abt" would not error anywhere -- it would
// just make the team invisible in every picker. Failing the deploy is the only
// place that mistake is cheap to catch.
var validFamilies = map[Family]bool{
	FamilyCREAbt: true,
	FamilyCRE:    true,
	FamilySREAbt: true,
	FamilySRE:    true,
	FamilySME:    true,
}

// Team is one of the organisation's teams. Team names are organisation
// vocabulary and are never hardcoded in this repo: the registry is supplied as
// deployment configuration (see ParseTeamRegistry) and resolved once at startup.
// The registry is a flat list; there is no sub-team nesting.
type Team struct {
	// Key is the registry key, and the id this platform exposes for the team.
	// It is stable across environments; the backing group's id is not.
	Key string
	// Name is the exact group name in the backing data source. It is matched
	// verbatim against the group name on a membership query, so an empty or
	// misspelt value silently resolves zero members -- ParseTeamRegistry
	// rejects an empty one for that reason.
	Name string
	// Family may be empty -- not every team has a family assigned.
	Family Family
	// CreGroupID is the backing data source's own id for this team's group, in
	// that source's compact 32-character form. It is distinct from Name: Name
	// is what membership resolution matches on, while CreGroupID is what backs
	// the case-search creTeam filter once converted to this platform's UUID
	// form. This is specifically the CRE (Customer Renewal & Expansion) group
	// id, as opposed to SreGroupID below, which backs the parallel sreTeam
	// filter -- the two disciplines have genuinely different backing groups,
	// so a team may have one, the other, or both. It is optional -- a team
	// with no configured id is still listed, it just cannot scope that
	// filter, which degrades gracefully rather than erroring.
	CreGroupID string
	// SreGroupID is the backing data source's own id for this team's Site
	// Reliability Engineering group, in the same compact 32-character form as
	// CreGroupID. It backs the case-search sreTeam filter. Like CreGroupID it
	// is optional and independent -- a team like an SRE ABT may configure only
	// this one, with no CreGroupID at all.
	SreGroupID string
}

// ParseTeamRegistry parses the team registry from its flat, single-line
// configuration form:
//
//	teamKey|Display Name|FAMILY|creGroupId|sreGroupId,teamKey|Display Name,...
//
// Rows are separated by ",", fields within a row by "|". A row carries two
// fields (key and display name), three (plus the family), four (plus the
// backing CRE group's id), or five (plus the backing SRE group's id). family
// is a real optional middle field, not a slot that can be skipped: a group id
// cannot be supplied without a family alongside it, so a 2-field-plus-id shape
// is not accepted -- pad the family field (even empty, e.g. "key|Name||id")
// if a team needs an id but no family. Likewise, an sreGroupId cannot be
// supplied without a creGroupId slot alongside it (even empty, e.g.
// "key|Name|FAMILY||sreId") since fields are positional. Whitespace around
// every field is trimmed, so a value pasted into a web form survives. A
// wholly blank row is skipped, which tolerates a trailing comma.
//
// The single-line shape is deliberate: the deployment platform's configuration
// UI is one-dimensional and stringifies nested collections, so a structured
// (nested-array) registry cannot be deployed at all. Do not reintroduce one.
//
// An empty string yields no teams and no error. Any malformed row is an error
// naming the offending row, so a typo stops a deploy instead of silently
// degrading team resolution at the first request.
func ParseTeamRegistry(raw string) ([]Team, error) {
	rows := strings.Split(raw, ",")
	teams := make([]Team, 0, len(rows))

	for i, row := range rows {
		if strings.TrimSpace(row) == "" {
			continue
		}

		fields := strings.Split(row, "|")
		if len(fields) < 2 || len(fields) > 5 {
			return nil, fmt.Errorf(
				"team registry row %d (%q): expected 2, 3, 4, or 5 %q-separated fields (teamKey|displayName[|family[|creGroupId[|sreGroupId]]]), got %d",
				i+1, strings.TrimSpace(row), "|", len(fields))
		}
		for j := range fields {
			fields[j] = strings.TrimSpace(fields[j])
		}

		if fields[0] == "" {
			return nil, fmt.Errorf("team registry row %d (%q): teamKey is empty", i+1, strings.TrimSpace(row))
		}
		// An empty display name is the dangerous one: it is matched verbatim
		// against the group name upstream, so it would resolve zero members
		// without any error surfacing anywhere.
		if fields[1] == "" {
			return nil, fmt.Errorf("team registry row %d (%q): displayName is empty", i+1, strings.TrimSpace(row))
		}

		team := Team{Key: fields[0], Name: fields[1]}
		if len(fields) >= 3 {
			family, err := parseFamily(fields[2])
			if err != nil {
				return nil, fmt.Errorf("team registry row %d (%q): %w", i+1, strings.TrimSpace(row), err)
			}
			team.Family = family
		}
		if len(fields) >= 4 {
			// Validate a supplied group id for the same reason the fields above
			// are validated: sourceIDToUUID passes anything that is not exactly
			// 32 hex characters through unchanged, so a typo (a 31-character id,
			// a stray character) yields a malformed value that matches nothing
			// on the creTeam filter without erroring anywhere. An absent id
			// stays legal -- that team just cannot scope the filter.
			if err := validateGroupID(fields[3]); err != nil {
				return nil, fmt.Errorf("team registry row %d (%q): %w", i+1, strings.TrimSpace(row), err)
			}
			team.CreGroupID = fields[3]
		}
		if len(fields) == 5 {
			// Same validation, same reasoning, for the parallel sreTeam filter.
			if err := validateGroupID(fields[4]); err != nil {
				return nil, fmt.Errorf("team registry row %d (%q): %w", i+1, strings.TrimSpace(row), err)
			}
			team.SreGroupID = fields[4]
		}
		teams = append(teams, team)
	}

	return teams, nil
}

// validateGroupID rejects a configured group id (CRE or SRE) that is not the
// backing data source's compact 32-hex-character form. An empty value is
// legal: the id is optional, and a team without one is still listed, it just
// cannot scope the corresponding case-search filter.
func validateGroupID(id string) error {
	if id == "" {
		return nil
	}
	if len(id) != 32 || !isHex(id) {
		return fmt.Errorf(
			"groupId %q is not a 32-character hexadecimal id (got %d character(s))", id, len(id))
	}
	return nil
}

// parseFamily normalizes a configured family value (in any case, e.g.
// "SRE-ABT") into this package's lowercase Family constants. An empty value is
// legal and yields the empty family: not every team is classified.
//
// Any other value is an error rather than being passed through: the dashboard
// team picker and default-dashboard selection both branch on the family, so an
// unrecognised value silently removes a team from every picker instead of
// erroring anywhere. A rejected deploy is the cheaper failure.
func parseFamily(family string) (Family, error) {
	trimmed := strings.TrimSpace(family)
	if trimmed == "" {
		return "", nil
	}
	normalized := Family(strings.ToLower(trimmed))
	if !validFamilies[normalized] {
		return "", fmt.Errorf(
			"unknown family %q: expected one of %q, %q, %q, %q, %q, or empty",
			trimmed, FamilyCREAbt, FamilyCRE, FamilySREAbt, FamilySRE, FamilySME)
	}
	return normalized, nil
}
