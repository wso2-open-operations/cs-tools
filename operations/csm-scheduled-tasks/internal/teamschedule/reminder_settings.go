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

// ReminderWindow is a rotation's hours on its own clock: "08:00" to "18:00"
// in America/Los_Angeles.
type ReminderWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	TimeZone string `json:"timeZone"`
}

// ReminderSettings is what the rota admins set for a family's reminders on
// the Team Schedule's Rota Access tab (entity-service's
// team_schedule_reminder_setting).
type ReminderSettings struct {
	WeekdayEnabled  bool                      `json:"weekdayEnabled"`
	WeekendEnabled  bool                      `json:"weekendEnabled"`
	InvitesEnabled  bool                      `json:"invitesEnabled"`
	WeekendRequired map[string]int            `json:"weekendRequired"`
	Windows         map[string]ReminderWindow `json:"windows"`
	// InviteRotations is, per rotation (the shift code of its window),
	// whether its reminders carry a calendar invitation. A rotation not named
	// does.
	InviteRotations map[string]bool `json:"inviteRotations"`
	CopyTo          []string        `json:"copyTo"`
	// WeekendCopyTo is copied on the weekend summary email only.
	WeekendCopyTo []string `json:"weekendCopyTo"`
	// InviteCopyTo is copied on the emails that carry a calendar invitation
	// or its cancellation, and on no other. Absent from an older
	// entity-service, which reads as none.
	InviteCopyTo []string `json:"inviteCopyTo"`
}

// ReminderSettingsFor is the family's saved reminder settings, or nil when
// nothing has been saved (the task then keeps its own configuration).
func (c *Client) ReminderSettingsFor(ctx context.Context, family string) (*ReminderSettings, error) {
	body, err := c.do(ctx, http.MethodGet, "/team-schedule/reminder-settings/"+url.PathEscape(family), nil)
	if notFoundSaying(err, noSettingsMessage) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s ReminderSettings
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("teamschedule: decode reminder settings: %w", err)
	}
	return &s, nil
}

// ReminderRequest is a rota reminder asked for by hand ("Send now" on the
// Rota Access tab), claimed for sending.
type ReminderRequest struct {
	ID          string `json:"id"`
	Family      string `json:"family"`
	Kind        string `json:"kind"`
	Test        bool   `json:"test"`
	RequestedBy string `json:"requestedBy"`
}

func reminderRequestPath(family string) string {
	return "/team-schedule/reminder-requests/" + url.PathEscape(family)
}

// ClaimReminderRequest claims the family's oldest waiting "Send now" request,
// or returns nil when nothing is waiting.
func (c *Client) ClaimReminderRequest(ctx context.Context, family string) (*ReminderRequest, error) {
	body, err := c.do(ctx, http.MethodPost, reminderRequestPath(family)+"/claim", nil)
	if notFoundSaying(err, nothingWaitingMessage) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var req ReminderRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("teamschedule: decode reminder request: %w", err)
	}
	return &req, nil
}

// ReportReminderRequest records how a claimed request went: SENT or FAILED.
func (c *Client) ReportReminderRequest(ctx context.Context, family, id, status, result string) error {
	body, err := json.Marshal(map[string]string{"status": status, "result": result})
	if err != nil {
		return fmt.Errorf("teamschedule: encode reminder request result: %w", err)
	}
	_, err = c.do(ctx, http.MethodPut, reminderRequestPath(family)+"/"+url.PathEscape(id)+"/result", body)
	return err
}
