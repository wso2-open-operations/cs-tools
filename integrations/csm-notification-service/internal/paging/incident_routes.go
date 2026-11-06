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

package paging

import (
	"os"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// IncidentRouteTable is where SRE incidents' "Incident Reported" cards, and the
// paging replies under them, go according to the group -> team mapping
// (sre.teams.aliases, which the Alert Settings routes fill) and each SRE
// team's room (sre.teamChats, which the Alert Settings rooms fill).
//
// Spaces maps incident audiences to webhook URLs:
//   - every SRE team key with a room: a group named as the key needs no alias;
//   - every aliased group whose team has a room;
//   - every group with a room of its own (sre.groupChats, saved on the
//     Alert Settings tab), which wins over the two above;
//   - notifications.IncidentFallbackAudience: the ladder's shared room
//     (sre.chat), for a group that maps to no room.
//
// Fanout maps each default group (sre.teams.defaultGroups: an incident with
// no SRE team of its own) to the sre.teamlessChats team keys, so its card goes
// to the rooms its paging cards already go to -- unless the group has a room
// of its own, which then takes its card.
type IncidentRouteTable struct {
	Spaces map[string]string
	Fanout map[string][]string
	// Mapped is the number of assignment groups (aliases and team keys)
	// that resolved to a room, for the start-up log line.
	Mapped int
	// Teamless is the sre.teamlessChats team keys with a room: where an S0
	// case whose account has no SRE team gets its card.
	Teamless []string
}

// BuildIncidentRoutes builds the IncidentRouteTable from the SRE ladder's merged
// configuration. url resolves a chat section to its webhook URL; a section it
// cannot resolve is left out.
func BuildIncidentRoutes(l LadderConfig, url func(Chat) (string, bool)) IncidentRouteTable {
	out := IncidentRouteTable{Spaces: map[string]string{}, Fanout: map[string][]string{}}
	teamURL := map[string]string{}
	for key, c := range l.TeamChats {
		u, ok := url(c)
		if !ok {
			continue
		}
		k := teamKeyFor(key)
		teamURL[k] = u
		out.Spaces[notifications.IncidentAudience(k)] = u
		out.Mapped++
	}
	for group, team := range l.Teams.Aliases {
		if u, ok := teamURL[teamKeyFor(team)]; ok {
			out.Spaces[notifications.IncidentAudience(group)] = u
			out.Mapped++
		}
	}
	ownRoom := map[string]bool{}
	for group, c := range l.GroupChats {
		key := teamKeyFor(group)
		u, ok := url(c)
		if key == "" || !ok {
			continue
		}
		if _, mapped := out.Spaces[notifications.IncidentAudience(key)]; !mapped {
			out.Mapped++
		}
		out.Spaces[notifications.IncidentAudience(key)] = u
		ownRoom[key] = true
	}
	var teamless []string
	for _, k := range l.TeamlessChats {
		if _, ok := teamURL[teamKeyFor(k)]; ok {
			teamless = append(teamless, teamKeyFor(k))
		}
	}
	out.Teamless = teamless
	if len(teamless) > 0 {
		for _, g := range l.Teams.DefaultGroups {
			if key := teamKeyFor(g); key != "" && !ownRoom[key] {
				out.Fanout[key] = teamless
			}
		}
	}
	if u, ok := url(l.Chat); ok {
		out.Spaces[notifications.IncidentFallbackAudience] = u
	}
	return out
}

// ChatWebhookURL resolves a chat section the way the ladder does
// (ladderChat): its webhookUrlEnv variable when named, else its audience
// through shared. An empty section resolves to nothing.
func ChatWebhookURL(c Chat, shared interface {
	AudienceWebhookURL(string) (string, bool)
}, getenv func(string) string) (string, bool) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if name := strings.TrimSpace(c.WebhookURLEnv); name != "" {
		u := strings.TrimSpace(getenv(name))
		return u, u != ""
	}
	audience := strings.TrimSpace(c.Audience)
	if audience == "" || shared == nil {
		return "", false
	}
	return shared.AudienceWebhookURL(audience)
}
