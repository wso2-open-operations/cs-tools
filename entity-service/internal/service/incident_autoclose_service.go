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
	"strings"
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
	// Largest dailyLimit accepted; case auto-closure caps its own the same way
	// (a multiple of its page size).
	incidentAutocloseMaxDailyLimit = 100000
)

// incidentCloser is the part of IncidentService the auto-close uses to close an
// incident: the ordinary update, so the close is the same write (and the same
// mirror to the backing data source in dual-write) as any other.
type incidentCloser interface {
	UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error)
}

type incidentAutocloseService struct {
	repo        repository.IncidentAutocloseRepository
	incidents   incidentCloser
	access      AccessService
	systemEmail string
	// now is the clock the daily budget's UTC midnight comes from; tests set it.
	now func() time.Time
}

// NewIncidentAutocloseService constructs the Postgres-backed
// IncidentAutocloseService. incidents is the deployment's incident service,
// which closes each incident (as case auto-closure closes through the case
// service). systemEmail is the system user the close is attributed to, the
// same AUTOCLOSURE_SYSTEM_USER_EMAIL case auto-closure uses: optional, and
// empty (or blank) acts as domain.AutoclosureDefaultActor ("System user")
// with no user row, so closed_by_id stays NULL. access refuses anyone but an
// internal caller, as case auto-closure's service does on top of its route gate.
func NewIncidentAutocloseService(repo repository.IncidentAutocloseRepository, incidents incidentCloser, access AccessService, systemEmail string) IncidentAutocloseService {
	systemEmail = strings.TrimSpace(systemEmail)
	if systemEmail == "" {
		systemEmail = domain.AutoclosureDefaultActor
	}
	return &incidentAutocloseService{repo: repo, incidents: incidents, access: access, systemEmail: systemEmail, now: time.Now}
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
	if req.DailyLimit < 0 || req.DailyLimit > incidentAutocloseMaxDailyLimit {
		return 0, &apierror.ValidationError{Msg: "dailyLimit must be between 0 and 100000"}
	}
	switch {
	case req.Limit == 0:
		return incidentAutocloseDefaultLimit, nil
	case req.Limit < 0 || req.Limit > incidentAutocloseMaxLimit:
		return 0, &apierror.ValidationError{Msg: "limit must be between 1 and 1000"}
	}
	return req.Limit, nil
}

// Autoclose implements IncidentAutocloseService. Each incident is closed by
// the incident service's own update (state Closed, as the system user), in its
// own transaction, so one that fails is reported in Failed and does not stop
// the rest. The update re-checks under the row lock that the incident is still
// due, and skips one that is not. It adds no comment or work note; the update
// writes the activity entry and, in dual-write, mirrors the close to the
// backing data source like any other incident update.
//
// Daily budget. A positive DailyLimit caps how many incidents may enter
// CLOSED per UTC day. The budget is the DailyLimit minus the incidents whose
// closed_on is at or after 00:00 UTC today. That counts EVERY close today,
// manual ones included, not only auto-closes: it is the conservative reading
// of a ceiling, needs no extra column, and avoids closed_by_id, which is NULL
// when the system user has no user row. The page limit is clamped to what is
// left, and a real run re-reads the count before each close (as case
// auto-closure re-checks before each act), so overlapping runs overshoot by at
// most a few. A dry run writes nothing but applies the same budget, so its
// count is what a real run would close.
func (s *incidentAutocloseService) Autoclose(ctx context.Context, req domain.IncidentAutocloseRequest) (domain.IncidentAutocloseResponse, error) {
	// The route is internalOnly too; this is the second line case auto-closure
	// also keeps (autoclosureService.requireInternal): the run closes incidents,
	// and only an internal caller may start it.
	if err := RequireInternalCaller(ctx, s.access, "this resource is only available to internal callers"); err != nil {
		return domain.IncidentAutocloseResponse{}, err
	}
	limit, err := validateIncidentAutocloseRequest(req)
	if err != nil {
		return domain.IncidentAutocloseResponse{}, err
	}
	days := *req.ResolvedForDays
	budgeted := req.DailyLimit > 0
	var dayStart time.Time
	used := 0 // incidents closed since dayStart, as last read (plus this call's closes)
	if budgeted {
		now := s.now().UTC()
		dayStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		if used, err = s.repo.CountIncidentsClosedSince(ctx, dayStart); err != nil {
			return domain.IncidentAutocloseResponse{}, err
		}
		remaining := req.DailyLimit - used
		if remaining <= 0 {
			slog.InfoContext(ctx, "incident auto-close run: daily limit already reached", "dryRun", req.DryRun,
				"dailyLimit", req.DailyLimit, "closedToday", used)
			return domain.IncidentAutocloseResponse{
				DryRun: req.DryRun, Closed: []domain.IncidentAutocloseClosed{}, Failed: []domain.IncidentAutocloseFailed{},
				DailyLimitReached: true,
			}, nil
		}
		limit = min(limit, remaining)
	}
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
	reached := false
	closed := domain.IncidentStateClosed
	closeCtx := withIncidentAutoclose(ctx, s.systemEmail, days)
	for _, c := range due {
		if req.DryRun {
			resp.Closed = append(resp.Closed, autoclosedEntry(c))
			used++
			continue
		}
		// A request that ran out of time stops here; what closed stays closed
		// and the next run picks up the rest.
		if err := ctx.Err(); err != nil {
			return domain.IncidentAutocloseResponse{}, err
		}
		if budgeted {
			if used, err = s.repo.CountIncidentsClosedSince(ctx, dayStart); err != nil {
				return resp, err
			}
			if used >= req.DailyLimit {
				break
			}
		}
		_, err = s.incidents.UpdateIncident(closeCtx, domain.UpdateIncidentRequest{ID: c.ID, State: &closed})
		if errors.Is(err, repository.ErrIncidentNotDueForAutoclose) {
			// Re-opened, closed or touched since it was listed: no longer due.
			continue
		}
		if err != nil {
			slog.ErrorContext(ctx, "incident auto-close: close failed", "incidentId", c.ID, "number", c.Number, "error", err)
			resp.Failed = append(resp.Failed, domain.IncidentAutocloseFailed{ID: c.ID, Number: c.Number, Error: autocloseFailureMessage(err)})
			continue
		}
		resp.Closed = append(resp.Closed, autoclosedEntry(c))
		used++
	}
	// The budget is used up: tell the caller, and stop it paging (more would
	// only make it call again to be answered with an empty page).
	if budgeted && used >= req.DailyLimit {
		reached = true
		resp.More = false
	}
	resp.DailyLimitReached = reached
	slog.InfoContext(ctx, "incident auto-close run", "dryRun", req.DryRun, "resolvedForDays", days,
		"dailyLimit", req.DailyLimit, "dailyLimitReached", reached,
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
