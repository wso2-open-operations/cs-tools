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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const alertMuteRulesNotOnServiceNowMsg = "alert mute rules are not supported on the ServiceNow data source"

// snAlertMuteRuleService answers every alert mute rule route with a 400 on the ServiceNow data source: alert mute rules exist only in
// Postgres (migration 0278). alert-core treats the refused GET /alert-mute-rules/active as "no rules" (it fails open).
type snAlertMuteRuleService struct{}

// NewSNAlertMuteRuleService constructs the ServiceNow-mode AlertMuteRuleService, which refuses every call.
func NewSNAlertMuteRuleService() AlertMuteRuleService { return snAlertMuteRuleService{} }

func snAlertMuteRulesUnsupported() error {
	return &apierror.ValidationError{Msg: alertMuteRulesNotOnServiceNowMsg}
}

func (snAlertMuteRuleService) Search(context.Context, domain.SearchAlertMuteRulesRequest) (domain.SearchAlertMuteRulesResponse, error) {
	return domain.SearchAlertMuteRulesResponse{}, snAlertMuteRulesUnsupported()
}

func (snAlertMuteRuleService) Get(context.Context, string) (domain.AlertMuteRule, error) {
	return domain.AlertMuteRule{}, snAlertMuteRulesUnsupported()
}

func (snAlertMuteRuleService) Active(context.Context) (domain.ActiveAlertMuteRulesResponse, error) {
	return domain.ActiveAlertMuteRulesResponse{}, snAlertMuteRulesUnsupported()
}

func (snAlertMuteRuleService) Create(context.Context, domain.SaveAlertMuteRuleRequest) (domain.AlertMuteRule, error) {
	return domain.AlertMuteRule{}, snAlertMuteRulesUnsupported()
}

func (snAlertMuteRuleService) Update(context.Context, string, domain.SaveAlertMuteRuleRequest) (domain.AlertMuteRule, error) {
	return domain.AlertMuteRule{}, snAlertMuteRulesUnsupported()
}

func (snAlertMuteRuleService) Extend(context.Context, string, domain.ExtendAlertMuteRuleRequest) (domain.AlertMuteRule, error) {
	return domain.AlertMuteRule{}, snAlertMuteRulesUnsupported()
}

func (snAlertMuteRuleService) End(context.Context, string) (domain.AlertMuteRule, error) {
	return domain.AlertMuteRule{}, snAlertMuteRulesUnsupported()
}

func (snAlertMuteRuleService) Delete(context.Context, string) error {
	return snAlertMuteRulesUnsupported()
}
