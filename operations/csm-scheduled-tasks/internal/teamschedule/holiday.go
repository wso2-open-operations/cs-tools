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

package teamschedule

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// HolidayMember is a CRE member a Public holiday tag was placed for.
type HolidayMember struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	// Status is TAGGED (off), AVAILABLE (works) or AWAY (another tag).
	Status   string `json:"status"`
	AwayKind string `json:"awayKind,omitempty"`
}

// HolidayTurn is a rota turn that day with nobody available on it.
type HolidayTurn struct {
	ShiftLabel string `json:"shiftLabel"`
	Name       string `json:"name"`
}

// HolidayAvailability is one CRE team's availability on one holiday.
type HolidayAvailability struct {
	TeamKey     string          `json:"teamKey"`
	TeamName    string          `json:"teamName"`
	Date        string          `json:"date"`
	Titles      []string        `json:"titles"`
	Countries   []string        `json:"countries"`
	Status      string          `json:"status"`
	FullyMarked bool            `json:"fullyMarked"`
	Members     []HolidayMember `json:"members"`
	Turns       int             `json:"turns"`
	Uncovered   []HolidayTurn   `json:"uncovered"`
	ConfirmedBy string          `json:"confirmedBy,omitempty"`
}

// HolidayRecipient is a team lead or rota manager a reminder goes to.
type HolidayRecipient struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// DueHolidayReminder is one reminder entity-service says is due today.
type DueHolidayReminder struct {
	Stage        string              `json:"stage"`
	DaysBefore   int                 `json:"daysBefore"`
	Availability HolidayAvailability `json:"availability"`
	Recipients   []HolidayRecipient  `json:"recipients"`
}

// DueHolidayReminders asks entity-service which holiday availability
// reminders are due on today (YYYY-MM-DD, the rota's calendar).
func (c *Client) DueHolidayReminders(ctx context.Context, today string) ([]DueHolidayReminder, error) {
	body, err := json.Marshal(struct {
		Today string `json:"today"`
	}{today})
	if err != nil {
		return nil, fmt.Errorf("teamschedule: encode holiday reminders request: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPost, "/team-schedule/holiday-availability/reminders/due", body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Reminders []DueHolidayReminder `json:"reminders"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("teamschedule: decode holiday reminders: %w", err)
	}
	return resp.Reminders, nil
}

// RecordHolidayReminder tells entity-service a reminder went out, so it is
// not due again.
func (c *Client) RecordHolidayReminder(ctx context.Context, teamKey, date, stage string) error {
	_, err := c.do(ctx, http.MethodPut, "/team-schedule/holiday-availability/reminders/"+
		url.PathEscape(teamKey)+"/"+url.PathEscape(date)+"/"+url.PathEscape(stage), []byte(`{}`))
	return err
}
