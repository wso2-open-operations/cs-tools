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

import "strings"

// PortalLinks builds the portal link an escalation card points at.
//
// This used to come from recipientlinks.Resolver.IncidentLink, which was
// removed when incident.created stopped posting a Google Chat alert -- nothing
// else needed an incident link any more. The ladder still does: its card is
// asking somebody to go and look at the incident, so it has to say where.
//
// It is a handful of string formatting rather than a reinstated dependency,
// and it is deliberately here rather than back in recipientlinks: that package
// resolves a link per recipient by role, and this has no recipient to resolve
// against -- a rung's card goes to a room.
type PortalLinks struct {
	// CSMBaseURL is the CSM portal's own base, e.g. https://csm.wso2.com.
	CSMBaseURL string
}

// IncidentLink implements the notifier's incidentLinker. An empty base yields
// an empty link, which the card treats as "no link" rather than rendering a
// broken one.
func (l PortalLinks) IncidentLink(incidentID string) string {
	base := strings.TrimRight(strings.TrimSpace(l.CSMBaseURL), "/")
	if base == "" || incidentID == "" {
		return ""
	}
	return base + "/operations/incidents/" + incidentID
}

// CaseLink is the CSM portal's page for a customer case -- the same
// "/cases/<id>" the case.* Chat cards open (recipientlinks.Resolver.CSMLink).
func (l PortalLinks) CaseLink(caseID string) string {
	base := strings.TrimRight(strings.TrimSpace(l.CSMBaseURL), "/")
	if base == "" || caseID == "" {
		return ""
	}
	return base + "/cases/" + caseID
}
