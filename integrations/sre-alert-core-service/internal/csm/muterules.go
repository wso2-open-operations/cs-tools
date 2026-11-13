// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package csm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AlertMuteRuleCondition is one condition of an alert mute rule: the alert's Field (service, environment, source,
// metric_name, severity, category or unique_identifier) must match Value.
type AlertMuteRuleCondition struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// AlertMuteRule is the subset of entity-service's AlertMuteRule (a one-time alert mute rule) this service reads.
type AlertMuteRule struct {
	ID         string                   `json:"id"`
	Number     string                   `json:"number"`
	Name       string                   `json:"name"`
	StartOn    time.Time                `json:"startOn"`
	EndOn      time.Time                `json:"endOn"`
	Conditions []AlertMuteRuleCondition `json:"conditions"`
}

type activeAlertMuteRulesResponse struct {
	AlertMuteRules []AlertMuteRule `json:"alertMuteRules"`
}

// ActiveAlertMuteRules returns the alert mute rules active right now (not ended, started, not yet expired).
func (c *Client) ActiveAlertMuteRules(ctx context.Context) ([]AlertMuteRule, error) {
	body, err := c.do(ctx, http.MethodGet, "/alert-mute-rules/active", nil)
	if err != nil {
		return nil, err
	}
	return decodeActiveAlertMuteRules(body)
}

func decodeActiveAlertMuteRules(body []byte) ([]AlertMuteRule, error) {
	var resp activeAlertMuteRulesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("csm: decode active alert mute rules: %w", err)
	}
	return resp.AlertMuteRules, nil
}
