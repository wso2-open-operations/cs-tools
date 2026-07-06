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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

type snTaskSlaSearchPayload struct {
	Filters    *snTaskSlaFilters   `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snTaskSlaFilters struct {
	TaskIDs []string `json:"taskIds,omitempty"`
}

type snTaskSlasResponse struct {
	TaskSlas     []snTaskSla `json:"slas"`
	TotalRecords int         `json:"totalRecords"`
	Limit        int         `json:"limit"`
	Offset       int         `json:"offset"`
}

type snTaskSla struct {
	ID                        string               `json:"id"`
	SlaDefinition             *snTaskSlaDefinition `json:"slaDefinition"`
	Stage                     *snTaskSlaStage      `json:"stage"`
	Task                      *snTaskSlaTaskRef    `json:"task"`
	BusinessTimeLeft          *string              `json:"businessTimeLeft"`
	BusinessElapsedTime       *string              `json:"businessElapsedTime"`
	BusinessElapsedPercentage *float64             `json:"businessElapsedPercentage"`
	StartTime                 *string              `json:"startTime"`
	EndTime                   *string              `json:"endTime"`
}

type snTaskSlaTaskRef struct {
	ID   *string `json:"id"`
	Name *string `json:"name"`
	Type *string `json:"type"`
}

type snTaskSlaDefinition struct {
	ID     *string `json:"id"`
	Name   *string `json:"name"`
	Type   *string `json:"type"`
	Target *string `json:"target"`
}

type snTaskSlaStage struct {
	ID    *string `json:"id"`
	Label *string `json:"label"`
}

type snTaskSlaDetail struct {
	ID                        string                    `json:"id"`
	Task                      *snTaskSlaTaskRef         `json:"task"`
	SlaDefinition             *snTaskSlaDefinitionDetail `json:"slaDefinition"`
	Stage                     *snTaskSlaStage           `json:"stage"`
	BusinessTimeLeft          *string                   `json:"businessTimeLeft"`
	BusinessElapsedTime       *string                   `json:"businessElapsedTime"`
	BusinessElapsedPercentage *float64                  `json:"businessElapsedPercentage"`
	StartTime                 *string                   `json:"startTime"`
	EndTime                   *string                   `json:"endTime"`
	Active                    *bool                     `json:"active"`
	Schedule                  *snTaskSlaScheduleRef     `json:"schedule"`
}

type snTaskSlaDefinitionDetail struct {
	ID               *string `json:"id"`
	Name             *string `json:"name"`
	Type             *string `json:"type"`
	Target           *string `json:"target"`
	Flow             *string `json:"flow"`
	Workflow         *string `json:"workflow"`
	EnableLogging    *bool   `json:"isEnableLogging"`
	DurationType     *string `json:"durationType"`
	Duration         *string `json:"duration"`
	ScheduleSource   *string `json:"scheduleSource"`
	Schedule         *string `json:"schedule"`
	TimezoneSource   *string `json:"timezoneSource"`
	Timezone         *string `json:"timezone"`
	StartCondition   *string `json:"startCondition"`
	RetroactiveStart *bool   `json:"isRetroactiveStart"`
	RetroactivePause *bool   `json:"isRetroactivePause"`
	WhenToCancel     *string `json:"whenToCancel"`
	CancelCondition  *string `json:"cancelCondition"`
	PauseCondition   *string `json:"pauseCondition"`
	WhenToResume     *string `json:"whenToResume"`
	StopCondition    *string `json:"stopCondition"`
	ResetCondition   *string `json:"resetCondition"`
	ResetAction      *string `json:"resetAction"`
}

type snTaskSlaScheduleRef struct {
	ID       *string `json:"id"`
	Name     *string `json:"name"`
	Timezone *string `json:"timezone"`
}

func snTaskSlaToView(t snTaskSla) domain.TaskSlaView {
	view := domain.TaskSlaView{
		ID:                        sysidToUUID(t.ID),
		BusinessTimeLeft:          t.BusinessTimeLeft,
		BusinessElapsedTime:       t.BusinessElapsedTime,
		BusinessElapsedPercentage: t.BusinessElapsedPercentage,
		StartTime:                 t.StartTime,
		EndTime:                   t.EndTime,
	}

	if t.SlaDefinition != nil {
		def := &domain.TaskSlaDefinition{
			Name:   t.SlaDefinition.Name,
			Type:   t.SlaDefinition.Type,
			Target: t.SlaDefinition.Target,
		}
		if t.SlaDefinition.ID != nil {
			id := sysidToUUID(*t.SlaDefinition.ID)
			def.ID = &id
		}
		view.SlaDefinition = def
	}

	if t.Stage != nil {
		view.Stage = t.Stage.Label
	}

	if t.Task != nil {
		ref := &domain.TaskSlaTaskRef{
			Name: t.Task.Name,
			Type: t.Task.Type,
		}
		if t.Task.ID != nil {
			id := sysidToUUID(*t.Task.ID)
			ref.ID = &id
		}
		view.Task = ref
	}

	return view
}

type snTaskSlaService struct {
	client *integrationservice.Client
}

// NewServiceNowTaskSlaService constructs a TaskSlaService backed by ServiceNow via integrationservice.Client.
func NewServiceNowTaskSlaService(client *integrationservice.Client) TaskSlaService {
	return &snTaskSlaService{client: client}
}

func snTaskSlaToDetailView(t snTaskSlaDetail) domain.TaskSlaDetail {
	view := domain.TaskSlaDetail{
		ID:                        sysidToUUID(t.ID),
		BusinessTimeLeft:          t.BusinessTimeLeft,
		BusinessElapsedTime:       t.BusinessElapsedTime,
		BusinessElapsedPercentage: t.BusinessElapsedPercentage,
		StartTime:                 t.StartTime,
		EndTime:                   t.EndTime,
		Active:                    t.Active,
	}

	if t.Task != nil {
		ref := &domain.TaskSlaTaskRef{
			Name: t.Task.Name,
			Type: t.Task.Type,
		}
		if t.Task.ID != nil {
			id := sysidToUUID(*t.Task.ID)
			ref.ID = &id
		}
		view.Task = ref
	}

	if t.SlaDefinition != nil {
		def := &domain.TaskSlaDefinitionDetail{
			Name:             t.SlaDefinition.Name,
			Type:             t.SlaDefinition.Type,
			Target:           t.SlaDefinition.Target,
			Flow:             t.SlaDefinition.Flow,
			Workflow:         t.SlaDefinition.Workflow,
			IsEnableLogging:  t.SlaDefinition.EnableLogging,
			DurationType:     t.SlaDefinition.DurationType,
			Duration:         t.SlaDefinition.Duration,
			ScheduleSource:   t.SlaDefinition.ScheduleSource,
			Schedule:         t.SlaDefinition.Schedule,
			TimezoneSource:   t.SlaDefinition.TimezoneSource,
			Timezone:         t.SlaDefinition.Timezone,
			StartCondition:   t.SlaDefinition.StartCondition,
			IsRetroactiveStart: t.SlaDefinition.RetroactiveStart,
			IsRetroactivePause: t.SlaDefinition.RetroactivePause,
			WhenToCancel:     t.SlaDefinition.WhenToCancel,
			CancelCondition:  t.SlaDefinition.CancelCondition,
			PauseCondition:   t.SlaDefinition.PauseCondition,
			WhenToResume:     t.SlaDefinition.WhenToResume,
			StopCondition:    t.SlaDefinition.StopCondition,
			ResetCondition:   t.SlaDefinition.ResetCondition,
			ResetAction:      t.SlaDefinition.ResetAction,
		}
		if t.SlaDefinition.ID != nil {
			id := sysidToUUID(*t.SlaDefinition.ID)
			def.ID = &id
		}
		view.SlaDefinition = def
	}

	if t.Stage != nil {
		view.Stage = t.Stage.Label
	}

	if t.Schedule != nil {
		ref := &domain.TaskSlaScheduleRef{
			Name:     t.Schedule.Name,
			Timezone: t.Schedule.Timezone,
		}
		if t.Schedule.ID != nil {
			id := sysidToUUID(*t.Schedule.ID)
			ref.ID = &id
		}
		view.Schedule = ref
	}

	return view
}

func (s *snTaskSlaService) GetTaskSla(ctx context.Context, id string) (domain.TaskSlaDetail, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.TaskSlaDetail{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.TaskSlaDetail{}, err
	}

	raw, err := s.client.Get(ctx, "/slas/"+uuidToSysid(id), token)
	if err != nil {
		return domain.TaskSlaDetail{}, err
	}

	var t snTaskSlaDetail
	if err := json.Unmarshal(raw, &t); err != nil {
		return domain.TaskSlaDetail{}, fmt.Errorf("sn get task sla: parse response: %w", err)
	}

	return snTaskSlaToDetailView(t), nil
}

func (s *snTaskSlaService) SearchTaskSlas(ctx context.Context, req domain.SearchTaskSlasRequest) (domain.SearchTaskSlasResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchTaskSlasResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchTaskSlasResponse{}, err
	}

	payload := snTaskSlaSearchPayload{
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	if req.Filters != nil {
		if err := validateUUIDs("taskIds", req.Filters.TaskIDs); err != nil {
			return domain.SearchTaskSlasResponse{}, err
		}
		payload.Filters = &snTaskSlaFilters{
			TaskIDs: uuidsToSysids(req.Filters.TaskIDs),
		}
	}

	raw, err := s.client.Post(ctx, "/slas/search", token, payload)
	if err != nil {
		return domain.SearchTaskSlasResponse{}, err
	}

	var snResp snTaskSlasResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchTaskSlasResponse{}, fmt.Errorf("sn task slas: parse response: %w", err)
	}

	views := make([]domain.TaskSlaView, 0, len(snResp.TaskSlas))
	for _, t := range snResp.TaskSlas {
		views = append(views, snTaskSlaToView(t))
	}

	return domain.SearchTaskSlasResponse{
		TaskSlas: views,
		Total:    snResp.TotalRecords,
		Limit:    snResp.Limit,
		Offset:   snResp.Offset,
	}, nil
}
