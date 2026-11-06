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

package dispatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// isCatastrophic reports whether a case severity is S0, in any of the forms
// entity-service and the paging configuration use for it.
func isCatastrophic(severity string) bool {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "CATASTROPHIC", "S0", "P0":
		return true
	}
	return false
}

// isCustomerCase reports whether a case type is a customer case, the only
// type the paging ladders page (paging/cases.go makes the same check).
func isCustomerCase(caseType string) bool {
	return strings.EqualFold(strings.TrimSpace(caseType), "case")
}

// caseSRERooms is where an S0 case's SRE room card goes: its account SRE
// group's rooms when that group is a known SRE team, else the teamless rooms
// (notifications.CaseSREAudiences, which the SRE ladder's replies use too).
// The SRE ladder posts nothing until a rung finds someone to page, so without
// this card the SRE room hears of an S0 only minutes later. None for any
// other severity.
func (d *Dispatcher) caseSRERooms(ctx context.Context, t events.Type, caseID, severity, sreGroup string) []string {
	if !isCatastrophic(severity) {
		return nil
	}
	audiences := notifications.CaseSREAudiences(d.incidentRoutes, d.incidentChatFanout, sreGroup, d.googleChat.HasAudienceSpace)
	if len(audiences) == 0 {
		slog.InfoContext(ctx, "dispatch: S0 case has no SRE room (no team room, teamless room or shared SRE room), no SRE room card",
			"type", string(t), "caseId", caseID, "sreGroup", sreGroup)
	}
	return audiences
}

// caseSRERoomCardBudget bounds the whole SRE room step, every room together.
// The handler waits for the step before it releases the record's keys, and a
// Kafka partition takes its next record only after that, which may be another
// case's CRE event. Without a bound, a hanging webhook would hold the
// partition for the Chat client's 10 s timeout per room (about 30 s for three
// teamless rooms). Normally a room takes about a second. A variable so a test
// can shorten it.
var caseSRERoomCardBudget = 6 * time.Second

// startCaseSRERoomCard posts an S0 case's card to its SRE rooms on its own
// goroutine and returns a channel that closes when it is done. It runs beside
// the rest of the record so it delays nothing there (the SLA clocks, emails
// and Incident Monitor card), and it is best effort: a failure is logged,
// never returned, so it cannot retry the record and resend those. The caller
// waits on the channel before releasing the record's keys.
//
// The rooms are posted one after another, not at once: two rooms can share a
// Chat space (the teamless rooms often do), and Chat answers a burst to one
// space with 429. The whole step has caseSRERoomCardBudget; a room not reached
// by then is left out and logged, like any other failed room.
func (d *Dispatcher) startCaseSRERoomCard(ctx context.Context, t events.Type, caseID string, rooms []string, card notifications.IncidentCreatedAlert) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(ctx, caseSRERoomCardBudget)
		defer cancel()
		var errs []error
		for _, room := range rooms {
			if err := d.googleChat.SendIncidentCreatedAlert(ctx, room, card); err != nil {
				errs = append(errs, err)
			}
		}
		if err := errors.Join(errs...); err != nil {
			slog.WarnContext(ctx, "dispatch: S0 case card did not reach every SRE room",
				"type", string(t), "caseId", caseID, "rooms", rooms, "errSummary", chatErrSummary(err))
		}
	}()
	return done
}

// closedDone is a done channel for a record with no SRE room card to wait on.
var closedDone = func() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}()

// caseSRERoomCard is the card an S0 case posts to its SRE rooms: the
// incident's "Reported" card in its case form, keyed by the case id so the
// SRE ladder's rung cards reply under it (paging's sreCaseCard), as an
// incident's do.
//
// The case is named as the SRE ladder's replies name it (paging's
// Trigger.caseRef): the WSO2 case ID, else the case number.
func caseSRERoomCard(caseID, caseNumber, wso2CaseID, sreGroup, severity, title, product, project, caseLink string) notifications.IncidentCreatedAlert {
	ref := strings.TrimSpace(wso2CaseID)
	if ref == "" {
		ref = displayCaseRef(caseNumber, caseID)
	}
	return notifications.IncidentCreatedAlert{
		IncidentID:       caseID,
		Number:           ref,
		Team:             sreGroup,
		Priority:         severity,
		ShortDescription: truncateTitle(title, maxChatTitleLength),
		Service:          product,
		Category:         "case",
		IncidentLink:     caseLink,
		Case:             true,
		Project:          project,
	}
}

// chatErrSummary is a Chat send error safe to log: the HTTP status of each
// failed post, or the error's type. Never the error text, which for a failed
// post carries an excerpt of Google Chat's response body and so can echo
// case content.
func chatErrSummary(err error) string {
	var parts []string
	var walk func(error)
	walk = func(err error) {
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			for _, e := range joined.Unwrap() {
				walk(e)
			}
			return
		}
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) {
			parts = append(parts, fmt.Sprintf("status %d", apiErr.StatusCode))
			return
		}
		parts = append(parts, fmt.Sprintf("%T", err))
	}
	if err != nil {
		walk(err)
	}
	return strings.Join(parts, "; ")
}
