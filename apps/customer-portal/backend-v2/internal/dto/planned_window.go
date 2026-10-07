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

package dto

import (
	"errors"
	"fmt"
	"time"
)

// A change request's planned window (plannedStartOn / plannedEndOn on PATCH,
// plannedStartDate / plannedEndDate on create) is checked here BEFORE the request
// goes to entity-service, so a typing mistake is a readable 400 from this API
// instead of a round trip.
//
// This is the first layer, never the only one: entity-service parses the window
// again with the same two layouts and the same year bounds
// (change_request_window.go there) and is what binds it into SQL. The rules are
// copied, not loosened, and the messages are the ones entity-service uses, so the
// webapp's own mapping of a 400 (changeRequests.ts describeChangeRequestActionError)
// reads either layer's answer the same way.
//
// Two layouts are accepted, and nothing else:
//
//   - RFC 3339 with a zone designator ("2030-03-01T09:00:00Z", "...+05:30"): an
//     instant as written;
//   - "YYYY-MM-DD HH:MM:SS" with no zone ("2030-03-01 09:00:00"): UTC. That is
//     the layout both portals send.
const (
	plannedZonelessLayout = "2006-01-02 15:04:05"

	// plannedYearMin / plannedYearMax bound a plausible implementation window,
	// as entity-service does.
	plannedYearMin = 2000
	plannedYearMax = 2100
)

// PlannedWindowError is a refusal of a planned window that the handler answers
// with a 400 carrying Message.
type PlannedWindowError struct{ Message string }

func (e *PlannedWindowError) Error() string { return e.Message }

// IsPlannedWindowError reports whether err is a *PlannedWindowError.
func IsPlannedWindowError(err error) bool {
	var target *PlannedWindowError
	return errors.As(err, &target)
}

func invalidPlannedTimestamp(field string) error {
	return &PlannedWindowError{Message: fmt.Sprintf(
		"%s must be a valid date-time, either RFC 3339 (2030-03-01T09:00:00Z) or YYYY-MM-DD HH:MM:SS in UTC, in the years %d to %d",
		field, plannedYearMin, plannedYearMax)}
}

// parsePlannedTimestamp parses one planned start / end into an instant.
func parsePlannedTimestamp(field, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t, err = time.Parse(plannedZonelessLayout, value)
	}
	if err != nil {
		return time.Time{}, invalidPlannedTimestamp(field)
	}
	if y := t.UTC().Year(); y < plannedYearMin || y > plannedYearMax {
		return time.Time{}, invalidPlannedTimestamp(field)
	}
	return t.UTC(), nil
}

// PlannedWindowRules are the rules a window is held to.
type PlannedWindowRules struct {
	// Now is the instant "the past" is measured from. Used only with
	// RequireFuture.
	Now time.Time
	// RequireFuture refuses a start or an end that is not after Now. Only a
	// customer's proposed time is held to it (a proposal is for a time still to
	// come); WSO2 staff re-planning a change may have a reason to record one that
	// has gone.
	RequireFuture bool
	// RequireOrder refuses a request whose start is not before its end, when it
	// carries both. entity-service applies this to a customer's proposal and to a
	// Re-schedule, not to a plain edit, so only the customer's proposal asks for
	// it here: this layer never refuses what entity-service would accept.
	RequireOrder bool
}

// ValidatePlannedWindow checks the window a request carries: each bound that is
// present must be a well-formed date-time in range (and, with RequireFuture, still
// to come), and, with RequireOrder, when both are present the start must be before
// the end.
//
// A bound that is nil is not checked and is not a mistake: a request may move one
// side of the stored window, and what that does to the other side is for
// entity-service, which has the stored value. The messages are entity-service's.
func ValidatePlannedWindow(startField string, start *string, endField string, end *string, rules PlannedWindowRules) error {
	var startAt, endAt time.Time
	for _, p := range []struct {
		field string
		value *string
		out   *time.Time
	}{{startField, start, &startAt}, {endField, end, &endAt}} {
		if p.value == nil {
			continue
		}
		t, err := parsePlannedTimestamp(p.field, *p.value)
		if err != nil {
			return err
		}
		if rules.RequireFuture && !t.After(rules.Now) {
			return &PlannedWindowError{Message: fmt.Sprintf(
				"%s is in the past: a proposed implementation time must be one still to come", p.field)}
		}
		*p.out = t
	}
	if rules.RequireOrder && start != nil && end != nil {
		switch {
		case startAt.After(endAt):
			return &PlannedWindowError{Message: "the planned start must not be after the planned end"}
		case startAt.Equal(endAt):
			return &PlannedWindowError{Message: "the planned start must not be the same as the planned end: the window must have a duration"}
		}
	}
	return nil
}
