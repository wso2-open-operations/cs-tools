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

package acp

import (
	"testing"
	"time"
)

func TestScreeningApplies(t *testing.T) {
	// 2026-07-27 is a Monday in Colombo (and UTC); 07-28 a Tuesday.
	monday := time.Date(2026, 7, 27, 4, 0, 0, 0, time.UTC)
	tuesday := monday.AddDate(0, 0, 1)
	// 20:00 UTC on Sunday is already Monday 01:30 in Colombo.
	sundayEveningUTC := time.Date(2026, 7, 26, 20, 0, 0, 0, time.UTC)
	on := Config{Screening: true, FullSweepWeekday: time.Monday}
	for _, tc := range []struct {
		name      string
		cfg       Config
		now       time.Time
		want      bool
		wantWhyIs bool
	}{
		{"screening off", Config{FullSweepWeekday: time.Monday}, tuesday, false, false},
		{"on, an ordinary day", on, tuesday, true, false},
		{"on, the full sweep weekday", on, monday, false, true},
		{"on, the weekday is judged in Colombo time", on, sundayEveningUTC, false, true},
		{"on, --full-sweep", Config{Screening: true, FullSweepWeekday: time.Monday, FullSweep: true}, tuesday, false, true},
		{"on, single project", Config{Screening: true, FullSweepWeekday: time.Monday, TestProjectID: "p"}, tuesday, false, true},
		{"on, another full sweep weekday", Config{Screening: true, FullSweepWeekday: time.Friday}, tuesday, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := screeningApplies(tc.cfg, tc.now)
			if got != tc.want || (why != "") != tc.wantWhyIs {
				t.Errorf("apply=%v why=%q, want %v / why given %v", got, why, tc.want, tc.wantWhyIs)
			}
		})
	}
}
