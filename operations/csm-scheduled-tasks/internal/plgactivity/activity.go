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

// Package plgactivity summarises one organisation's Agent Manager events after
// signup. It holds no I/O.
package plgactivity

import (
	"regexp"
	"sort"
	"time"
)

// SessionGap ends a session: 30 minutes without an event.
const SessionGap = 30 * time.Minute

// setupActions are emitted by the platform minutes after signup (dev data,
// 2026-11): counted, but not as the user's actions.
var setupActions = map[string]bool{"registered-gateway": true, "rotated-gateway-token": true}

// productAction is an Agent Manager action (created-agent, deleted-agent).
// Account events such as "Select Subscription Plan" share the platform tag.
var productAction = regexp.MustCompile(`^[a-z]+(-[a-z]+)+$`)

// Event is one Agent Manager event, by time and action only.
type Event struct {
	At     time.Time
	Action string
}

// Activity is what the events say about one registration.
type Activity struct {
	FirstEventOn, LastEventOn *time.Time
	EventCount                int            // every event
	ActionCounts              map[string]int // every event, by action
	Actions                   int            // the user's Agent Manager actions
	SessionMinutes            *int           // the user's first session; nil without any
}

// Derive summarises events inside [from, to).
func Derive(events []Event, from, to time.Time) Activity {
	a := Activity{ActionCounts: map[string]int{}}
	var user []Event
	for _, e := range events {
		if e.At.Before(from) || !e.At.Before(to) || e.Action == "" {
			continue
		}
		a.EventCount++
		a.ActionCounts[e.Action]++
		if a.FirstEventOn == nil || e.At.Before(*a.FirstEventOn) {
			a.FirstEventOn = ptr(e.At)
		}
		if a.LastEventOn == nil || e.At.After(*a.LastEventOn) {
			a.LastEventOn = ptr(e.At)
		}
		if productAction.MatchString(e.Action) && !setupActions[e.Action] {
			user = append(user, e)
		}
	}
	a.Actions = len(user)
	if len(user) == 0 {
		return a
	}
	sort.SliceStable(user, func(i, j int) bool { return user[i].At.Before(user[j].At) })
	end := user[0].At
	for i := 1; i < len(user) && user[i].At.Sub(user[i-1].At) <= SessionGap; i++ {
		end = user[i].At
	}
	a.SessionMinutes = ptr(int(end.Sub(user[0].At) / time.Minute))
	return a
}

func ptr[T any](v T) *T { return &v }
