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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snAlert mirrors the backing service's GET /custom-alerts/{id} response.
type snAlert struct {
	ID          string  `json:"sysId"`
	Number      *string `json:"number"`
	Environment *string `json:"environment"`
	MetricName  *string `json:"metricName"`
	Source      *string `json:"source"`
	Category    *string `json:"category"`
	Severity    *string `json:"severity"`
	Description *string `json:"description"`
	IncidentID  *string `json:"incidentSysId"`
	ServiceID   *string `json:"serviceSysId"`
	CreatedOn   string  `json:"createdOn"`
}

func snAlertToView(a snAlert) domain.AlertView {
	view := domain.AlertView{
		Number:      a.Number,
		Environment: a.Environment,
		MetricName:  a.MetricName,
		Source:      a.Source,
		Category:    a.Category,
		Severity:    a.Severity,
		Description: a.Description,
		CreatedOn:   a.CreatedOn,
	}
	if a.ID != "" {
		id := sysidToUUID(a.ID)
		view.ID = &id
	}
	if a.IncidentID != nil {
		id := sysidToUUID(*a.IncidentID)
		view.IncidentID = &id
	}
	if a.ServiceID != nil {
		id := sysidToUUID(*a.ServiceID)
		view.ServiceID = &id
	}
	return view
}

type snAlertService struct {
	client *integrationservice.Client
}

// NewServiceNowAlertService constructs an AlertService backed by the Choreo API.
func NewServiceNowAlertService(client *integrationservice.Client) AlertService {
	return &snAlertService{client: client}
}

func (s *snAlertService) GetAlertByID(ctx context.Context, id string) (domain.AlertView, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AlertView{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/custom-alerts/"+uuidToSysid(id), token)
	if err != nil {
		return domain.AlertView{}, err
	}

	var a snAlert
	if err := json.Unmarshal(raw, &a); err != nil {
		return domain.AlertView{}, fmt.Errorf("sn get alert: parse response: %w", err)
	}

	return snAlertToView(a), nil
}
