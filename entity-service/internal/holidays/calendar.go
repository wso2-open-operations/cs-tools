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

package holidays

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// maxHolidaySpanDays bounds how many days one calendar event may expand to,
// so a mis-entered event cannot flood the table.
const maxHolidaySpanDays = 31

// Holiday is one public holiday on one day.
type Holiday struct {
	// Date is the day, "2006-01-02".
	Date  string
	Title string
}

// holidayGroup is the calendar-events service's response element (the leave
// app's HolidayGroup): a country and its Google Calendar events.
type holidayGroup struct {
	Country  string          `json:"country"`
	Holidays []calendarEvent `json:"holidays"`
}

type calendarEvent struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Start   struct {
		Date string `json:"date"`
	} `json:"start"`
	End struct {
		Date string `json:"date"`
	} `json:"end"`
}

// CalendarClient reads public holidays from the calendar-events service.
type CalendarClient struct {
	baseURL string
	tokens  *tokenSource
}

// NewCalendarClient builds a CalendarClient for the calendar-events service at
// baseURL.
func NewCalendarClient(baseURL string, creds Credentials) *CalendarClient {
	return &CalendarClient{baseURL: strings.TrimRight(baseURL, "/"), tokens: newTokenSource(creds)}
}

// Holidays returns country's public holidays between from and to (inclusive
// days), one entry per day, as the leave app reads them.
func (c *CalendarClient) Holidays(ctx context.Context, country string, from, to time.Time) ([]Holiday, error) {
	q := url.Values{}
	q.Set("countries", country)
	q.Set("startTimeMin", from.UTC().Format(dateLayout)+"T00:00:00Z")
	q.Set("startTimeMax", to.UTC().Format(dateLayout)+"T00:00:00Z")
	endpoint := c.baseURL + "/holidays?" + q.Encode()
	raw, err := c.tokens.do(ctx, "calendar-events /holidays", func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	})
	if err != nil {
		return nil, err
	}
	var groups []holidayGroup
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, fmt.Errorf("holidays: parse /holidays response: %w", err)
	}
	// The leave app reads only the first group: it asked for one country.
	if len(groups) == 0 {
		return []Holiday{}, nil
	}
	return expandEvents(groups[0].Holidays), nil
}

// expandEvents turns all-day calendar events into one Holiday per day, like
// the leave app's getHolidaysFromEvents: an all-day event's end date is the
// day after its last day. Events without both dates (timed events) are
// skipped, as there.
func expandEvents(events []calendarEvent) []Holiday {
	out := []Holiday{}
	for _, ev := range events {
		start, err := time.Parse(dateLayout, firstDate(ev.Start.Date))
		if err != nil {
			continue
		}
		end, err := time.Parse(dateLayout, firstDate(ev.End.Date))
		if err != nil {
			continue
		}
		title := strings.TrimSpace(ev.Summary)
		if title == "" {
			title = "Public holiday"
		}
		if !end.After(start) {
			end = start.AddDate(0, 0, 1)
		}
		for d, n := start, 0; d.Before(end) && n < maxHolidaySpanDays; d, n = d.AddDate(0, 0, 1), n+1 {
			out = append(out, Holiday{Date: d.Format(dateLayout), Title: title})
		}
	}
	return out
}

// firstDate keeps the "2006-01-02" part of a date that may carry a time.
func firstDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > len(dateLayout) {
		return s[:len(dateLayout)]
	}
	return s
}
