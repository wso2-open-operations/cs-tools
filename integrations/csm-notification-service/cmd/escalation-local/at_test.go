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

package main

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/escalation"
)

// The day matters as much as the hour: 10:00 is LK on a weekday and
// LK_WEEKEND on a Saturday. HH:MM must therefore never drift onto the wrong
// kind of day just because today's slot has passed -- a Friday evening run of
// "-at 10:00" must land on Monday, not Saturday.
func TestReportTime(t *testing.T) {
	// Friday 2026-10-02 20:00 IST.
	friEvening := time.Date(2026, 10, 2, 20, 0, 0, 0, escalation.IST)

	cases := []struct {
		name    string
		at      string
		weekend bool
		want    string // "Mon 15:04", or "" when an error is expected
	}{
		{"weekday slot already passed rolls to Monday, not Saturday", "10:00", false, "Mon 10:00"},
		{"weekday slot still ahead today stays today", "21:30", false, "Fri 21:30"},
		{"-weekend picks the next Saturday", "10:00", true, "Sat 10:00"},
		{"a full future date is taken as given", "2026-10-05T07:15", false, "Mon 07:15"},
		{"the space form is accepted too", "2026-10-05 07:15", false, "Mon 07:15"},
		{"a past date is refused", "2026-10-01T10:00", false, ""},
		{"not a time of day", "25:00", false, ""},
		{"nonsense", "tomorrow", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := reportTime(c.at, c.weekend, friEvening)
			if c.want == "" {
				if err == nil {
					t.Fatalf("reportTime(%q) = %s, want an error", c.at, got.Format("Mon 15:04"))
				}
				return
			}
			if err != nil {
				t.Fatalf("reportTime(%q): %v", c.at, err)
			}
			if g := got.Format("Mon 15:04"); g != c.want {
				t.Errorf("reportTime(%q, weekend=%v) = %s, want %s", c.at, c.weekend, g, c.want)
			}
		})
	}
}
