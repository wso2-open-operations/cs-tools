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

// snSmartAlert mirrors the backing service's GET /smart-alert-buffers/{id} response.
type snSmartAlert struct {
	ID               string  `json:"sysId"`
	AlertID          *string `json:"alertId"`
	SourceAlertID    *string `json:"sourceAlertId"`
	AlertStatus      *string `json:"alertStatus"`
	WindowStatus     *string `json:"windowStatus"`
	Severity         *string `json:"severity"`
	Urgency          *string `json:"urgency"`
	Impact           *string `json:"impact"`
	Category         *string `json:"category"`
	Source           *string `json:"source"`
	Environment      *string `json:"environment"`
	ResourceName     *string `json:"resourceName"`
	ShortDescription *string `json:"shortDescription"`
	Details          *string `json:"details"`
	MonitorURL       *string `json:"monitorUrl"`
	FiredAt          *string `json:"firedAt"`
	ReceivedAt       *string `json:"receivedAt"`
	FireCount        *int    `json:"fireCount"`
	IncidentID       *string `json:"incidentSysId"`
}

func snSmartAlertToView(a snSmartAlert) domain.SmartAlertView {
	view := domain.SmartAlertView{
		SourceAlertID:    a.SourceAlertID,
		AlertStatus:      a.AlertStatus,
		WindowStatus:     a.WindowStatus,
		Severity:         a.Severity,
		Urgency:          a.Urgency,
		Impact:           a.Impact,
		Category:         a.Category,
		Source:           a.Source,
		Environment:      a.Environment,
		ResourceName:     a.ResourceName,
		ShortDescription: a.ShortDescription,
		Details:          a.Details,
		MonitorURL:       a.MonitorURL,
		FiredAt:          a.FiredAt,
		ReceivedAt:       a.ReceivedAt,
		FireCount:        a.FireCount,
	}
	if a.ID != "" {
		id := sysidToUUID(a.ID)
		view.ID = &id
	}
	if a.AlertID != nil {
		id := sysidToUUID(*a.AlertID)
		view.AlertID = &id
	}
	if a.IncidentID != nil {
		id := sysidToUUID(*a.IncidentID)
		view.IncidentID = &id
	}
	return view
}

type snSmartAlertService struct {
	client *integrationservice.Client
}

// NewServiceNowSmartAlertService constructs a SmartAlertService backed by the Choreo API.
func NewServiceNowSmartAlertService(client *integrationservice.Client) SmartAlertService {
	return &snSmartAlertService{client: client}
}

func (s *snSmartAlertService) GetSmartAlertByID(ctx context.Context, id string) (domain.SmartAlertView, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SmartAlertView{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/smart-alert-buffers/"+uuidToSysid(id), token)
	if err != nil {
		return domain.SmartAlertView{}, err
	}

	var a snSmartAlert
	if err := json.Unmarshal(raw, &a); err != nil {
		return domain.SmartAlertView{}, fmt.Errorf("sn get smart alert: parse response: %w", err)
	}

	return snSmartAlertToView(a), nil
}
