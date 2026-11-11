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
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	maxAlertMuteRuleNameLength        = 255
	maxAlertMuteRuleDescriptionLength = 1024
	maxAlertMuteRuleValueLength       = 255
	// maxAlertMuteRuleWindow is the longest an alert mute rule may last, extensions included.
	maxAlertMuteRuleWindow = 72 * time.Hour
	// alertMuteRuleStartGrace lets a start of "now" through, submitted a little after it was picked.
	alertMuteRuleStartGrace = 60 * time.Second
)

// alertMuteRuleFields are the alert's canonical fields a condition may name.
var alertMuteRuleFields = map[string]bool{
	"service": true, "environment": true, "source": true, "metric_name": true,
	"severity": true, "category": true, "unique_identifier": true,
}

// alertMuteRuleRequiredFields must each have a condition, so a rule never mutes every service or every environment.
var alertMuteRuleRequiredFields = []string{"service", "environment"}

var alertMuteRuleSearchStates = map[domain.AlertMuteRuleState]bool{
	domain.AlertMuteRuleStateActive: true, domain.AlertMuteRuleStateScheduled: true, domain.AlertMuteRuleStateExpired: true,
}

type alertMuteRuleService struct {
	repo repository.AlertMuteRuleRepository
	now  func() time.Time
}

// NewAlertMuteRuleService constructs the Postgres AlertMuteRuleService.
func NewAlertMuteRuleService(repo repository.AlertMuteRuleRepository) AlertMuteRuleService {
	return &alertMuteRuleService{repo: repo, now: time.Now}
}

func alertMuteRuleNotFound() error { return &apierror.NotFoundError{Msg: "alert mute rule not found"} }

func (s *alertMuteRuleService) Search(ctx context.Context, req domain.SearchAlertMuteRulesRequest) (domain.SearchAlertMuteRulesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAlertMuteRulesResponse{}, err
	}
	for _, st := range req.Filters.States {
		if !alertMuteRuleSearchStates[st] {
			return domain.SearchAlertMuteRulesResponse{}, &apierror.ValidationError{Msg: "filters.states must be active, scheduled or expired"}
		}
	}
	list, total, err := s.repo.Search(ctx, req.Filters.States, req.Filters.SearchQuery, s.now(), req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchAlertMuteRulesResponse{}, err
	}
	return domain.SearchAlertMuteRulesResponse{AlertMuteRules: list, Total: total, Offset: req.Pagination.Offset, Limit: req.Pagination.Limit}, nil
}

func (s *alertMuteRuleService) Get(ctx context.Context, id string) (domain.AlertMuteRule, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AlertMuteRule{}, err
	}
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	if b == nil {
		return domain.AlertMuteRule{}, alertMuteRuleNotFound()
	}
	return *b, nil
}

func (s *alertMuteRuleService) Active(ctx context.Context) (domain.ActiveAlertMuteRulesResponse, error) {
	list, err := s.repo.Active(ctx, s.now())
	if err != nil {
		return domain.ActiveAlertMuteRulesResponse{}, err
	}
	return domain.ActiveAlertMuteRulesResponse{AlertMuteRules: list}, nil
}

func (s *alertMuteRuleService) Create(ctx context.Context, req domain.SaveAlertMuteRuleRequest) (domain.AlertMuteRule, error) {
	w, err := validateAlertMuteRule(req, s.now())
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	return s.repo.Create(ctx, actorOf(ctx), w)
}

func (s *alertMuteRuleService) Update(ctx context.Context, id string, req domain.PatchAlertMuteRuleRequest) (domain.AlertMuteRule, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	now := s.now()
	w, err := validateAlertMuteRule(mergeAlertMuteRulePatch(current, req), now)
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	b, err := s.repo.Update(ctx, actorOf(ctx), id, w, now)
	return s.guarded(ctx, id, b, err, "only a scheduled alert mute rule can be edited")
}

// mergeAlertMuteRulePatch lays the fields present in req over the stored rule.
func mergeAlertMuteRulePatch(current domain.AlertMuteRule, req domain.PatchAlertMuteRuleRequest) domain.SaveAlertMuteRuleRequest {
	merged := domain.SaveAlertMuteRuleRequest{
		Name:        current.Name,
		Description: current.Description,
		TimeZone:    current.TimeZone,
		StartOn:     current.StartOn.UTC().Format(time.RFC3339),
		EndOn:       current.EndOn.UTC().Format(time.RFC3339),
		Conditions:  current.Conditions,
	}
	if req.Name != nil {
		merged.Name = *req.Name
	}
	if req.Description != nil {
		merged.Description = *req.Description
	}
	if req.TimeZone != nil {
		merged.TimeZone = *req.TimeZone
	}
	if req.StartOn != nil {
		merged.StartOn = *req.StartOn
	}
	if req.EndOn != nil {
		merged.EndOn = *req.EndOn
	}
	if req.Conditions != nil {
		merged.Conditions = *req.Conditions
	}
	return merged
}

func (s *alertMuteRuleService) Extend(ctx context.Context, id string, req domain.ExtendAlertMuteRuleRequest) (domain.AlertMuteRule, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	endOn, err := parseAlertMuteRuleTime("endOn", req.EndOn)
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	now := s.now()
	const wrongState = "only a scheduled or active alert mute rule can be extended"
	if st := current.StateAt(now); st != domain.AlertMuteRuleStateScheduled && st != domain.AlertMuteRuleStateActive {
		return domain.AlertMuteRule{}, &apierror.ConflictError{Msg: wrongState}
	}
	if !endOn.After(current.EndOn) {
		return domain.AlertMuteRule{}, &apierror.ValidationError{Msg: "endOn must be later than the alert mute rule's current end"}
	}
	if endOn.Sub(current.StartOn) > maxAlertMuteRuleWindow {
		return domain.AlertMuteRule{}, &apierror.ValidationError{Msg: "an alert mute rule can last at most 72 hours from its start"}
	}
	b, err := s.repo.Extend(ctx, actorOf(ctx), id, endOn, now)
	if err != nil || b != nil {
		return s.guarded(ctx, id, b, err, wrongState)
	}
	// The guarded UPDATE matched nothing: the rule changed after it was read. Say which way, so a rule that can still
	// be extended (another extension moved its end) isn't reported as being in the wrong state.
	latest, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	if latest == nil {
		return domain.AlertMuteRule{}, alertMuteRuleNotFound()
	}
	if st := latest.StateAt(s.now()); st == domain.AlertMuteRuleStateScheduled || st == domain.AlertMuteRuleStateActive {
		return domain.AlertMuteRule{}, &apierror.ConflictError{Msg: "the alert mute rule changed while it was being extended; reload it and try again"}
	}
	return domain.AlertMuteRule{}, &apierror.ConflictError{Msg: wrongState}
}

func (s *alertMuteRuleService) End(ctx context.Context, id string) (domain.AlertMuteRule, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AlertMuteRule{}, err
	}
	b, err := s.repo.End(ctx, actorOf(ctx), id, s.now())
	return s.guarded(ctx, id, b, err, "only an active alert mute rule can be ended")
}

func (s *alertMuteRuleService) Delete(ctx context.Context, id string) error {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return err
	}
	ok, err := s.repo.Delete(ctx, id, s.now())
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	_, err = s.guarded(ctx, id, nil, nil, "only a scheduled alert mute rule can be deleted")
	return err
}

// guarded turns a state-guarded write's result into the response: the rule written, or, when the guard matched no row, a
// 404 if the rule does not exist and a 409 if it is in the wrong state.
func (s *alertMuteRuleService) guarded(ctx context.Context, id string, b *domain.AlertMuteRule, err error, wrongState string) (domain.AlertMuteRule, error) {
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	if b != nil {
		return *b, nil
	}
	exists, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AlertMuteRule{}, err
	}
	if exists == nil {
		return domain.AlertMuteRule{}, alertMuteRuleNotFound()
	}
	return domain.AlertMuteRule{}, &apierror.ConflictError{Msg: wrongState}
}

func parseAlertMuteRuleTime(field, v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
	if err != nil {
		return time.Time{}, &apierror.ValidationError{Msg: field + " must be an RFC 3339 timestamp"}
	}
	return t.UTC(), nil
}

// validateAlertMuteRule checks a create or edit body at now and returns it trimmed, with UTC times.
func validateAlertMuteRule(req domain.SaveAlertMuteRuleRequest, now time.Time) (domain.AlertMuteRuleWrite, error) {
	bad := func(msg string) (domain.AlertMuteRuleWrite, error) {
		return domain.AlertMuteRuleWrite{}, &apierror.ValidationError{Msg: msg}
	}
	w := domain.AlertMuteRuleWrite{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		TimeZone:    strings.TrimSpace(req.TimeZone),
	}
	if w.Name == "" {
		return bad("name is required")
	}
	if utf8.RuneCountInString(w.Name) > maxAlertMuteRuleNameLength {
		return bad(fmt.Sprintf("name must be at most %d characters", maxAlertMuteRuleNameLength))
	}
	if utf8.RuneCountInString(w.Description) > maxAlertMuteRuleDescriptionLength {
		return bad(fmt.Sprintf("description must be at most %d characters", maxAlertMuteRuleDescriptionLength))
	}
	if w.TimeZone == "" {
		return bad("timeZone is required")
	}
	// "Local" is the server's zone, not one a person entered.
	if _, err := time.LoadLocation(w.TimeZone); err != nil || w.TimeZone == "Local" {
		return bad("timeZone must be an IANA time zone, such as Asia/Colombo")
	}
	var err error
	if w.StartOn, err = parseAlertMuteRuleTime("startOn", req.StartOn); err != nil {
		return domain.AlertMuteRuleWrite{}, err
	}
	if w.EndOn, err = parseAlertMuteRuleTime("endOn", req.EndOn); err != nil {
		return domain.AlertMuteRuleWrite{}, err
	}
	switch {
	case !w.EndOn.After(w.StartOn):
		return bad("endOn must be later than startOn")
	case w.EndOn.Sub(w.StartOn) > maxAlertMuteRuleWindow:
		return bad("an alert mute rule can last at most 72 hours")
	case !w.EndOn.After(now):
		return bad("endOn must be in the future")
	case w.StartOn.Before(now.Add(-alertMuteRuleStartGrace)):
		return bad("startOn must not be in the past")
	}
	if w.Conditions, err = validateAlertMuteRuleConditions(req.Conditions); err != nil {
		return domain.AlertMuteRuleWrite{}, err
	}
	return w, nil
}

func validateAlertMuteRuleConditions(in []domain.AlertMuteRuleCondition) ([]domain.AlertMuteRuleCondition, error) {
	bad := func(msg string) ([]domain.AlertMuteRuleCondition, error) {
		return nil, &apierror.ValidationError{Msg: msg}
	}
	out := make([]domain.AlertMuteRuleCondition, 0, len(in))
	seen := map[string]bool{}
	for i, c := range in {
		field := strings.TrimSpace(c.Field)
		value := strings.TrimSpace(c.Value)
		if !alertMuteRuleFields[field] {
			return bad(fmt.Sprintf("conditions[%d].field must be one of service, environment, source, metric_name, severity, category, unique_identifier", i))
		}
		if seen[field] {
			return bad(fmt.Sprintf("conditions[%d]: the field %s is given more than once", i, field))
		}
		seen[field] = true
		if value == "" {
			return bad(fmt.Sprintf("conditions[%d].value is required", i))
		}
		if utf8.RuneCountInString(value) > maxAlertMuteRuleValueLength {
			return bad(fmt.Sprintf("conditions[%d].value must be at most %d characters", i, maxAlertMuteRuleValueLength))
		}
		out = append(out, domain.AlertMuteRuleCondition{Field: field, Value: value})
	}
	for _, f := range alertMuteRuleRequiredFields {
		if !seen[f] {
			return bad("conditions must include service and environment")
		}
	}
	return out, nil
}
