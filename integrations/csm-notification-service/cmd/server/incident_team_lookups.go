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

package main

import "sync"

// incidentTeamLookups calls release once both team lookups an incident SLA
// alert needs have loaded successfully at least once: the teams' SLA spaces
// (team.chat_webhook_url by key) and their group emails (team.group_email).
// Until then the SLA engine holds incident tiers, so one is sent late rather
// than with no card or no team email. Safe for concurrent use; release runs
// at most once.
type incidentTeamLookups struct {
	mu       sync.Mutex
	spaces   bool
	emails   bool
	released bool
	release  func()
}

func newIncidentTeamLookups(release func()) *incidentTeamLookups {
	return &incidentTeamLookups{release: release}
}

func (l *incidentTeamLookups) slaSpacesLoaded()   { l.loaded(true, false) }
func (l *incidentTeamLookups) groupEmailsLoaded() { l.loaded(false, true) }

func (l *incidentTeamLookups) loaded(spaces, emails bool) {
	l.mu.Lock()
	l.spaces = l.spaces || spaces
	l.emails = l.emails || emails
	fire := l.spaces && l.emails && !l.released
	if fire {
		l.released = true
	}
	l.mu.Unlock()
	if fire && l.release != nil {
		l.release()
	}
}
