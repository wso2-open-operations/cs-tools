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

package domain

import (
	"testing"
	"time"
)

func TestAlertMuteRuleStateAt(t *testing.T) {
	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	ended := start.Add(time.Hour)
	open := AlertMuteRule{StartOn: start, EndOn: end}
	endedEarly := AlertMuteRule{StartOn: start, EndOn: end, EndedOn: &ended}
	for _, tc := range []struct {
		name string
		b    AlertMuteRule
		now  time.Time
		want AlertMuteRuleState
	}{
		{"before start", open, start.Add(-time.Second), AlertMuteRuleStateScheduled},
		{"at start", open, start, AlertMuteRuleStateActive},
		{"just before end", open, end.Add(-time.Nanosecond), AlertMuteRuleStateActive},
		{"at end", open, end, AlertMuteRuleStateExpired},
		{"after end", open, end.Add(time.Hour), AlertMuteRuleStateExpired},
		{"ended early, inside the window", endedEarly, start.Add(90 * time.Minute), AlertMuteRuleStateEnded},
		{"ended early, after the window", endedEarly, end.Add(time.Hour), AlertMuteRuleStateEnded},
	} {
		if got := tc.b.StateAt(tc.now); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}
