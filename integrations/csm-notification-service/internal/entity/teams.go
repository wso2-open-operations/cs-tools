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

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetTeamChatWebhooks calls entity-service's internal-caller-only
// GET /teams/chat-webhooks and returns a map of team name -> Chat incoming
// webhook URL, for every team that has one configured (team.chat_webhook_url
// -- see that repo's own CLAUDE.md). This is the replacement for hand-listing
// a team's own space in this service's GOOGLE_CHAT_SPACES config: the team
// table is now the source of truth for a team's own space, and
// notifications.GoogleChatClient.SetTeamAudienceSpaces is refreshed
// periodically from this call (see cmd/server/main.go) rather than fetched
// once at startup, since an operator can add a team's webhook URL at any
// time without restarting this service.
//
// A team with a blank or duplicate name is skipped (a duplicate collapses to
// "no webhook" for that name, same defensive posture
// NewGoogleChatClient already applies to its own config-sourced spaces --
// never guess which of two conflicting entries is right).
func (c *CustomerEntityClient) GetTeamChatWebhooks(ctx context.Context) (map[string]string, error) {
	respBody, err := c.do(ctx, http.MethodGet, "/teams/chat-webhooks", nil)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Teams []struct {
			Name           string `json:"name"`
			ChatWebhookURL string `json:"chatWebhookUrl"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("entity: decode GetTeamChatWebhooks response: %w", err)
	}

	webhooks := make(map[string]string, len(parsed.Teams))
	seen := make(map[string]bool, len(parsed.Teams))
	for _, t := range parsed.Teams {
		name := strings.TrimSpace(t.Name)
		url := strings.TrimSpace(t.ChatWebhookURL)
		if name == "" || url == "" {
			continue
		}
		if seen[name] {
			webhooks[name] = ""
			continue
		}
		seen[name] = true
		webhooks[name] = url
	}
	return webhooks, nil
}

// GetTeamGroupEmails calls entity-service's internal-caller-only
// GET /teams/group-emails and returns a map of team.key -> group_email, for
// every team that has one configured (see that repo's own CLAUDE.md). Keyed
// by key, not name, unlike GetTeamChatWebhooks above — this backs
// slaengine.Engine's lookup of one specific, stable team (e.g.
// "americas_cre_team" for its SLA-breach overnight-coverage email), not a
// case's own, possibly-renamed display team.
//
// A team with a blank or duplicate key is skipped the same defensive way
// GetTeamChatWebhooks handles a blank/duplicate name.
func (c *CustomerEntityClient) GetTeamGroupEmails(ctx context.Context) (map[string]string, error) {
	respBody, err := c.do(ctx, http.MethodGet, "/teams/group-emails", nil)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Teams []struct {
			Key        string `json:"key"`
			GroupEmail string `json:"groupEmail"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("entity: decode GetTeamGroupEmails response: %w", err)
	}

	emails := make(map[string]string, len(parsed.Teams))
	seen := make(map[string]bool, len(parsed.Teams))
	for _, t := range parsed.Teams {
		key := strings.TrimSpace(t.Key)
		email := strings.TrimSpace(t.GroupEmail)
		if key == "" || email == "" {
			continue
		}
		if seen[key] {
			emails[key] = ""
			continue
		}
		seen[key] = true
		emails[key] = email
	}
	return emails, nil
}
