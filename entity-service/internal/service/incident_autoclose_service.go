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
	"errors"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	incidentAutocloseDefaultLimit = 200
	incidentAutocloseMaxLimit     = 1000
	// Ten years. Far past any real setting, and well inside what
	// NOW() - make_interval(days => n) can compute: a few million days runs
	// past timestamptz's range and Postgres would answer an opaque 500.
	incidentAutocloseMaxDays = 3650
)

type incidentAutocloseService struct {
	repo repository.IncidentAutocloseRepository
}

// NewIncidentAutocloseService constructs the Postgres-backed
// IncidentAutocloseService.
func NewIncidentAutocloseService(repo repository.IncidentAutocloseRepository) IncidentAutocloseService {
	return &incidentAutocloseService{repo: repo}
}

// unavailableIncidentAutocloseService stands in where the feature cannot run
// (DATA_SOURCE=servicenow), so the route answers an explicit 503 with the
// reason instead of a bare 404.
type unavailableIncidentAutocloseService struct{ reason string }

// NewUnavailableIncidentAutocloseService returns a service whose every call
// answers a 503 carrying reason.
func NewUnavailableIncidentAutocloseService(reason string) IncidentAutocloseService {
	return unavailableIncidentAutocloseService{reason: reason}
}

func (u unavailableIncidentAutocloseService) Autoclose(context.Context, domain.IncidentAutocloseRequest) (domain.IncidentAutocloseResponse, error) {
	return domain.IncidentAutocloseResponse{}, &apierror.ServiceUnavailableError{Msg: u.reason}
}

// validateIncidentAutocloseRequest checks the request and returns the limit
// to use.
func validateIncidentAutocloseRequest(req domain.IncidentAutocloseRequest) (int, error) {
	if req.ResolvedForDays == nil {
		return 0, &apierror.ValidationError{Msg: "resolvedForDays is required"}
	}
	if *req.ResolvedForDays < 1 || *req.ResolvedForDays > incidentAutocloseMaxDays {
		return 0, &apierror.ValidationError{Msg: "resolvedForDays must be between 1 and 3650"}
	}
	switch {
	case req.Limit == 0:
		return incidentAutocloseDefaultLimit, nil
	case req.Limit < 0 || req.Limit > incidentAutocloseMaxLimit:
		return 0, &apierror.ValidationError{Msg: "limit must be between 1 and 1000"}
	}
	return req.Limit, nil
}

// Autoclose implements IncidentAutocloseService. Each incident is closed in
// its own transaction, so one that fails is reported in Failed and does not
// stop the rest. Like ServiceNow's job it adds no comment or work note; the
// one activity-feed row (state Resolved -> Closed, by the system actor) is
// written after the close commits and only logged when it fails.
func (s *incidentAutocloseService) Autoclose(ctx context.Context, req domain.IncidentAutocloseRequest) (domain.IncidentAutocloseResponse, error) {
	limit, err := validateIncidentAutocloseRequest(req)
	if err != nil {
		return domain.IncidentAutocloseResponse{}, err
	}
	days := *req.ResolvedForDays
	due, err := s.repo.ListIncidentsDueForAutoclose(ctx, days, limit)
	if err != nil {
		return domain.IncidentAutocloseResponse{}, err
	}
	resp := domain.IncidentAutocloseResponse{
		DryRun: req.DryRun,
		Closed: []domain.IncidentAutocloseClosed{},
		Failed: []domain.IncidentAutocloseFailed{},
		More:   len(due) == limit,
	}
	for _, c := range due {
		if req.DryRun {
			resp.Closed = append(resp.Closed, autoclosedEntry(c))
			continue
		}
		// A request that ran out of time stops here; what closed stays closed
		// and the next run picks up the rest.
		if err := ctx.Err(); err != nil {
			return domain.IncidentAutocloseResponse{}, err
		}
		closed, err := s.repo.AutocloseIncident(ctx, c.ID, days, incidentSystemActorEmail)
		if err != nil {
			slog.ErrorContext(ctx, "incident auto-close: close failed", "incidentId", c.ID, "number", c.Number, "error", err)
			resp.Failed = append(resp.Failed, domain.IncidentAutocloseFailed{ID: c.ID, Number: c.Number, Error: autocloseFailureMessage(err)})
			continue
		}
		if !closed {
			// Re-opened, closed or touched since it was listed: no longer due.
			continue
		}
		resp.Closed = append(resp.Closed, autoclosedEntry(c))
		if err := s.repo.RecordIncidentFieldChanges(ctx, c.ID, []repository.IncidentFieldChange{{
			Field: "state", OldValue: incidentEnumLabel(strPtr("RESOLVED")), NewValue: incidentEnumLabel(strPtr("CLOSED")),
		}}, incidentSystemActorEmail); err != nil {
			slog.ErrorContext(ctx, "incident auto-close: record state change activity failed", "incidentId", c.ID, "error", err)
		}
	}
	slog.InfoContext(ctx, "incident auto-close run", "dryRun", req.DryRun, "resolvedForDays", days,
		"selected", len(due), "closed", len(resp.Closed), "failed", len(resp.Failed), "more", resp.More)
	return resp, nil
}

func autoclosedEntry(c repository.IncidentAutocloseCandidate) domain.IncidentAutocloseClosed {
	e := domain.IncidentAutocloseClosed{ID: c.ID, Number: c.Number}
	if c.ResolvedOn != nil {
		s := c.ResolvedOn.UTC().Format(time.RFC3339)
		e.ResolvedOn = &s
	}
	return e
}

// autocloseFailureMessage is what a failed entry tells the caller. Our own
// error types carry messages written for callers; anything else (a database
// error, which can name tables and columns) is only logged.
func autocloseFailureMessage(err error) string {
	var ve *apierror.ValidationError
	var nfe *apierror.NotFoundError
	var ce *apierror.ConflictError
	switch {
	case errors.As(err, &ve):
		return ve.Msg
	case errors.As(err, &nfe):
		return nfe.Msg
	case errors.As(err, &ce):
		return ce.Msg
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "the request ended before the incident could be closed"
	}
	return "internal error closing the incident (see the entity-service log)"
}
