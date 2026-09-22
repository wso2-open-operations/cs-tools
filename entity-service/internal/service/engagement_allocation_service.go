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
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// cycleStartDateLayout is the only accepted form of the cycleStartDate query
// parameter. A bare date, not a timestamp: the underlying columns are DATE,
// and accepting an instant would invite a timezone to creep into a boundary
// that has none.
const cycleStartDateLayout = "2006-01-02"

type engagementAllocationService struct {
	repo repository.EngagementAllocationRepository
	// publisher is nil when Event Hub is not configured — same convention as
	// caseService.publisher. A nil publisher means the update is still
	// written; only the notification is skipped, with a warning. Losing the
	// email is strictly better than refusing to record someone's update.
	publisher EventPublisherService
}

// NewEngagementAllocationService constructs an EngagementAllocationService.
// publisher may be nil (see engagementAllocationService.publisher).
func NewEngagementAllocationService(repo repository.EngagementAllocationRepository, publisher EventPublisherService) EngagementAllocationService {
	return &engagementAllocationService{repo: repo, publisher: publisher}
}

// wso2EmailSuffix gates who may receive an engagement status update.
//
// The ServiceNow original meant to enforce this: its action's script step
// looped the cc list, and on the first non-WSO2 address set an error output
// and threw. But the throw was inside that script's own try/catch, the catch
// logged and commented "Do nothing", and nothing downstream read the error
// output — so the Email step ran anyway and the update went to the external
// address regardless. This port enforces what that code was trying to say.
const wso2EmailSuffix = "@wso2.com"

// validateRecipients rejects the whole request if any address is external,
// rather than silently dropping the offending ones. A status update is
// internal content, and an author who typed an outside address needs to know
// it was refused — quietly removing it would look like it had been sent.
func validateRecipients(field string, addresses []string) ([]string, error) {
	cleaned := make([]string, 0, len(addresses))
	for _, raw := range addresses {
		addr := strings.ToLower(strings.TrimSpace(raw))
		if addr == "" {
			continue
		}
		if !strings.HasSuffix(addr, wso2EmailSuffix) {
			return nil, &apierror.ValidationError{
				Msg: field + " contains a non-WSO2 address: " + addr,
			}
		}
		cleaned = append(cleaned, addr)
	}
	return cleaned, nil
}

// mondayOf returns the Monday of t's own ISO week, which is the cycle a status
// update filed during that week covers.
func mondayOf(t time.Time) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
}

// CreateStatusUpdate files one weekly engagement status update and publishes
// the notification event for csm-notification-service to send.
//
// The write and the publish are deliberately NOT atomic, and the write wins:
// a publish failure is logged by the publisher (into event_publish_failures)
// and the update is still returned as created. The alternative — failing the
// request because an email could not be queued — would lose the thing the
// user actually did.
func (s *engagementAllocationService) CreateStatusUpdate(ctx context.Context, req domain.CreateEngagementStatusUpdateRequest) (domain.EngagementStatusUpdate, error) {
	if err := validateUUIDs("engagementId", []string{req.EngagementID}); err != nil {
		return domain.EngagementStatusUpdate{}, err
	}
	if err := validateUUIDs("authorId", []string{req.AuthorID}); err != nil {
		return domain.EngagementStatusUpdate{}, err
	}
	if req.AllocationID != nil {
		if err := validateUUIDs("allocationId", []string{*req.AllocationID}); err != nil {
			return domain.EngagementStatusUpdate{}, err
		}
	}
	if strings.TrimSpace(req.Subject) == "" {
		return domain.EngagementStatusUpdate{}, &apierror.ValidationError{Msg: "subject is required"}
	}
	if strings.TrimSpace(req.Content) == "" {
		return domain.EngagementStatusUpdate{}, &apierror.ValidationError{Msg: "content is required"}
	}

	// The mailing list is the Cc audience, not the To — see the field's own
	// doc comment. It is also the list the ServiceNow action's script step
	// meant to filter, so this is the same field that check was aimed at.
	cc, err := validateRecipients("mailingList", req.MailingList)
	if err != nil {
		return domain.EngagementStatusUpdate{}, err
	}
	if len(cc) == 0 {
		return domain.EngagementStatusUpdate{}, &apierror.ValidationError{Msg: "mailingList must contain at least one address"}
	}

	cycleStart := mondayOf(time.Now().UTC())
	if req.CycleStartDate != "" {
		parsed, err := time.Parse(cycleStartDateLayout, req.CycleStartDate)
		if err != nil {
			return domain.EngagementStatusUpdate{}, &apierror.ValidationError{
				Msg: "cycleStartDate must be a date in YYYY-MM-DD form",
			}
		}
		cycleStart = parsed
	}

	// Resolved before the insert so a bad engagement id fails as a 404
	// instead of a foreign-key 500.
	nctx, err := s.repo.EngagementContext(ctx, req.EngagementID, req.AuthorID)
	if err != nil {
		return domain.EngagementStatusUpdate{}, err
	}

	req.MailingList = cc
	created, err := s.repo.CreateStatusUpdate(ctx, req, cycleStart)
	if err != nil {
		return domain.EngagementStatusUpdate{}, err
	}

	if s.publisher == nil {
		slog.WarnContext(ctx, "no event publisher configured; engagement status update recorded but not emailed",
			"statusUpdateId", created.ID)
		return created, nil
	}
	// To is the author, Cc is the mailing list — the binding the ServiceNow
	// flow used. When the author has no address on file the mail would have
	// no primary recipient at all, so the list is promoted to To rather than
	// dropping the notification: the audience still gets the update, which is
	// the point of sending it.
	to := []string{nctx.AuthorEmail}
	ccAudience := cc
	if nctx.AuthorEmail == "" {
		slog.WarnContext(ctx, "status update author has no email; addressing the mailing list directly",
			"statusUpdateId", created.ID, "authorId", created.AuthorID)
		to = cc
		ccAudience = nil
	}

	payload, err := json.Marshal(events.EngagementStatusUpdateCreatedPayload{
		EngagementID:   created.EngagementID,
		EngagementName: nctx.EngagementName,
		AuthorName:     nctx.AuthorName,
		Subject:        created.Subject,
		Content:        created.Content,
		CycleStartDate: created.CycleStartDate,
		Recipients:     to,
		CcRecipients:   ccAudience,
	})
	if err != nil {
		slog.ErrorContext(ctx, "marshal engagement status update payload", "err", err, "statusUpdateId", created.ID)
		return created, nil
	}
	if err := s.publisher.Publish(ctx, events.TypeEngagementStatusUpdateCreated, created.ID, payload); err != nil {
		slog.ErrorContext(ctx, "publish engagement status update event", "err", err, "statusUpdateId", created.ID)
	}
	return created, nil
}

// StatusUpdateReminderRecipients validates cycleStartDate and returns the
// people who owe an update for that cycle.
func (s *engagementAllocationService) StatusUpdateReminderRecipients(ctx context.Context, cycleStartDate string) (domain.StatusUpdateReminderResponse, error) {
	if cycleStartDate == "" {
		return domain.StatusUpdateReminderResponse{}, &apierror.ValidationError{Msg: "cycleStartDate is required"}
	}
	// time.Parse with a date-only layout yields midnight UTC, which is what
	// the DATE columns compare against.
	cycleStart, err := time.Parse(cycleStartDateLayout, cycleStartDate)
	if err != nil {
		return domain.StatusUpdateReminderResponse{}, &apierror.ValidationError{Msg: "cycleStartDate must be a date in YYYY-MM-DD form"}
	}

	recipients, err := s.repo.StatusUpdateReminderRecipients(ctx, cycleStart)
	if err != nil {
		return domain.StatusUpdateReminderResponse{}, err
	}
	return domain.StatusUpdateReminderResponse{
		CycleStartDate: cycleStartDate,
		Count:          len(recipients),
		Recipients:     recipients,
	}, nil
}
