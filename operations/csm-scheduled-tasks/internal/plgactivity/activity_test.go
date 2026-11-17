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

package plgactivity

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

func ev(min int, action string) Event {
	return Event{At: t0.Add(time.Duration(min) * time.Minute), Action: action}
}

func TestDerive(t *testing.T) {
	window := 12 * time.Hour
	cases := []struct {
		name           string
		events         []Event
		count, actions int
		session        *int
	}{
		{name: "no events"},
		{
			name:   "platform setup and account events are not the user's actions",
			events: []Event{ev(1, "registered-gateway"), ev(1, "rotated-gateway-token"), ev(2, "Select Subscription Plan")},
			count:  3,
		},
		{
			name:   "deletions count as actions",
			events: []Event{ev(0, "created-agent"), ev(5, "deleted-agent"), ev(20, "created-agent")},
			count:  3, actions: 3, session: ptr(20),
		},
		{
			name:   "the first session ends at a 30-minute gap",
			events: []Event{ev(90, "created-monitor"), ev(0, "created-agent"), ev(25, "deployed-agent")},
			count:  3, actions: 3, session: ptr(25),
		},
		{
			name:   "events outside the window are ignored",
			events: []Event{ev(-5, "created-agent"), ev(1, "created-agent"), ev(int(window/time.Minute), "deployed-agent")},
			count:  1, actions: 1, session: ptr(0),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Derive(c.events, t0, t0.Add(window))
			if a.EventCount != c.count || a.Actions != c.actions {
				t.Fatalf("count=%d actions=%d, want %d %d", a.EventCount, a.Actions, c.count, c.actions)
			}
			if (a.SessionMinutes == nil) != (c.session == nil) || (c.session != nil && *a.SessionMinutes != *c.session) {
				t.Fatalf("session = %v, want %v", a.SessionMinutes, c.session)
			}
			if c.count > 0 && (a.FirstEventOn == nil || a.LastEventOn == nil) {
				t.Fatal("first and last event not set")
			}
		})
	}
}
