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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// srAcknowledgement is the comment ServiceNow's "Acknowledge SR and Notify
// Chat" action posts, word for word.
const srAcknowledgement = "Your service request has been received successfully and has entered our review process. " +
	"An assigned team member will evaluate the request and provide an update as soon as possible."

// srNoticeActor stamps the automation's own writes, as incident reports do.
const srNoticeActor = "system"

// SRNoticeService ports ServiceNow's "SR New Request - Acknowledge & Chat
// Alert" flow and publishes the sr.* events (events/service_request.go) to
// the operations topic. Everything it does is best effort: the SR or comment
// that triggered it is already committed, and a failure here is logged, never
// returned to the caller.
//
// ServiceNow's flow, from discovery scripts 73/74:
//
//  1. On a new SR whose account's SRE team is one it automates, set the SR's
//     assignment group to that team and post the "New Service Request" card.
//  2. If the card went out, add the acknowledgement comment, move the SR to
//     Open (setWorkflow(false), so no comment notification), and post the
//     "Acknowledged" card in the same thread.
//
// Here the writes are entity-service's and the cards are
// csm-notification-service's, so step 2 is gated on the team being
// automated (SR_ALERT_SRE_TEAM_IDS) rather than on the card having been sent
// -- the same condition, short of a transient Chat failure.
type SRNoticeService struct {
	repo      repository.SRNoticeRepository
	publisher EventPublisherService
	// alertTeams is SR_ALERT_SRE_TEAM_IDS, lowercased: account.sre_team_id
	// reads back lowercase.
	alertTeams map[string]bool
}

// NewSRNoticeService constructs an SRNoticeService. publisher is the
// operations-topic publisher (routes.go wires this service only when there is
// one); alertTeamIDs is SR_ALERT_SRE_TEAM_IDS.
func NewSRNoticeService(repo repository.SRNoticeRepository, publisher EventPublisherService, alertTeamIDs []string) *SRNoticeService {
	teams := make(map[string]bool, len(alertTeamIDs))
	for _, id := range alertTeamIDs {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			teams[id] = true
		}
	}
	return &SRNoticeService{repo: repo, publisher: publisher, alertTeams: teams}
}

// OnCreated runs the flow for a service request that was just created.
// Called only for a case of type service_request.
func (s *SRNoticeService) OnCreated(ctx context.Context, caseID string) {
	// The flow runs as the system, not as whoever raised the SR (often a
	// customer, who could not assign it).
	ctx = repository.WithSystemIdentity(ctx)
	sr, ok, err := s.repo.GetServiceRequest(ctx, caseID)
	if err != nil {
		slog.ErrorContext(ctx, "sr notices: read new service request failed", "caseId", caseID, "error", err)
		return
	}
	if !ok {
		return
	}

	automated := sr.SRETeamID != "" && s.alertTeams[strings.ToLower(sr.SRETeamID)]
	if automated {
		if err := s.repo.AssignToGroup(ctx, caseID, sr.SRETeamID, srNoticeActor); err != nil {
			slog.ErrorContext(ctx, "sr notices: assign service request to its SRE team failed", "caseId", caseID, "error", err)
		} else {
			sr.AssignmentGroupName = sr.SRETeamName
		}
	}

	s.publish(ctx, events.TypeSRCreated, caseID, events.SRCreatedPayload{
		SRRef:       srRef(sr),
		Description: sr.Description,
		State:       srStateLabel(sr.State),
		ProjectID:   sr.ProjectID,
		ProjectName: sr.ProjectName,
		CreatedBy:   sr.CreatedBy,
		CreatedOn:   sr.CreatedOn.UTC().Format(time.RFC3339),
	})

	if !automated {
		return
	}
	commentID, err := s.repo.Acknowledge(ctx, caseID, srNoticeActor, srAcknowledgement)
	if err != nil {
		slog.ErrorContext(ctx, "sr notices: acknowledge service request failed", "caseId", caseID, "error", err)
		return
	}
	slog.InfoContext(ctx, "sr notices: service request assigned and acknowledged", "caseId", caseID, "number", sr.Number)
	s.publish(ctx, events.TypeSRAcknowledged, caseID, events.SRAcknowledgedPayload{
		SRRef:     srRef(sr),
		CommentID: commentID,
	})
}

// OnComment publishes sr.comment_added for a comment or work note just added
// to a case, when that case is a service request. Activity entries are not
// journal comments and are skipped, as is a comment with no author email: the
// consumer requires one, and an event it rejects is retried into its
// dead-letter topic for nothing.
func (s *SRNoticeService) OnComment(ctx context.Context, caseID, commentID string, commentType domain.CommentType, content, authorEmail, authorName string, createdOn time.Time) {
	var t events.SRCommentType
	switch commentType {
	case domain.CommentTypeComment:
		t = events.SRCommentTypeComment
	case domain.CommentTypeWorkNote:
		t = events.SRCommentTypeWorkNote
	default:
		return
	}
	if s.publisher == nil || strings.TrimSpace(authorEmail) == "" {
		return
	}
	sr, ok, err := s.repo.GetServiceRequest(repository.WithSystemIdentity(ctx), caseID)
	if err != nil {
		slog.ErrorContext(ctx, "sr notices: read service request for comment failed", "caseId", caseID, "error", err)
		return
	}
	if !ok {
		return
	}
	s.publish(ctx, events.TypeSRCommentAdded, caseID, events.SRCommentAddedPayload{
		SRRef:       srRef(sr),
		CommentID:   commentID,
		CommentType: t,
		Content:     content,
		AuthorEmail: authorEmail,
		AuthorName:  authorName,
		Tags:        sr.Tags,
		CreatedOn:   createdOn.UTC().Format(time.RFC3339),
	})
}

func (s *SRNoticeService) publish(ctx context.Context, eventType events.Type, caseID string, payload any) {
	if s.publisher == nil {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "sr notices: encode payload failed", "type", eventType, "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, eventType, caseID, raw); err != nil {
		// The error is not logged: it can carry broker details, and Publish
		// has already recorded it in event_publish_failures.
		slog.ErrorContext(ctx, "sr notices: publish failed", "type", eventType, "caseId", caseID)
		return
	}
	slog.InfoContext(ctx, "sr notices: published", "type", eventType, "caseId", caseID)
}

func srRef(sr repository.ServiceRequest) events.SRRef {
	return events.SRRef{
		CaseID:              sr.ID,
		Number:              sr.Number,
		WSO2CaseID:          sr.WSO2CaseID,
		Subject:             sr.Subject,
		SRETeamID:           sr.SRETeamID,
		SRETeamName:         sr.SRETeamName,
		AssignmentGroupName: sr.AssignmentGroupName,
	}
}

// srStateLabel turns a service_request_state_enum label into words for the
// card: OPEN -> "Open", WAITING_ON_WSO2 -> "Waiting On WSO2".
func srStateLabel(state string) string {
	if state == "" {
		return ""
	}
	words := strings.Split(strings.ToLower(state), "_")
	for i, w := range words {
		if w == "wso2" {
			words[i] = "WSO2"
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
