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

// Package strategy decides what happens to each alert. A Strategy is a pure decision over an Input; a Chain asks its strategies in order and the first to decide wins, so a new way of grouping (RIA, say) is one more Strategy added to the chain.
package strategy

import (
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Outcome is what happens to one alert.
type Outcome string

const (
	// Open opens a new incident for the alert.
	Open Outcome = "open"
	// Duplicate adds the alert to its fingerprint's open incident as a Duplicate note.
	Duplicate Outcome = "duplicate"
	// Resolve adds a resolving alert to its fingerprint's open incident as an OK note.
	Resolve Outcome = "resolve"
	// Mute drops an alert an active mute rule covers.
	Mute Outcome = "mute"
	// Ignore drops an alert that needs nothing, such as a resolving alert with no open incident.
	Ignore Outcome = "ignore"
)

// Folds reports whether the outcome adds the alert to an existing incident.
func (o Outcome) Folds() bool { return o == Duplicate || o == Resolve }

// Group is a fingerprint's state: the incident its alerts currently fold into.
type Group struct {
	Fingerprint string
	IncidentID  string
	FirstSeenOn time.Time
	LastSeenOn  time.Time
	AlertCount  int
}

// Input is everything a strategy may look at; strategies do no I/O.
type Input struct {
	Event event.Event
	// Group is nil for a fingerprint with no incident yet.
	Group *Group
	// IncidentOpen reports whether Group's incident is still open.
	IncidentOpen bool
	MuteRules    []domain.AlertMuteRule
}

// HasOpenIncident reports whether the alert's fingerprint has an incident to fold into.
func (in Input) HasOpenIncident() bool { return in.Group != nil && in.IncidentOpen }

// Decision is a strategy's verdict on one alert.
type Decision struct {
	Outcome Outcome
	// Strategy names the strategy that decided, for logs.
	Strategy string
	// Reason says why, for logs.
	Reason string
}

// Strategy decides an alert's outcome, or passes with false.
type Strategy interface {
	Name() string
	Decide(in Input) (Decision, bool)
}

// Chain asks its strategies in order; the first to decide wins.
type Chain []Strategy

// Decide returns the first decision in the chain, or Open when no strategy decides.
func (c Chain) Decide(in Input) Decision {
	for _, s := range c {
		if d, ok := s.Decide(in); ok {
			d.Strategy = s.Name()
			return d
		}
	}
	return Decision{Outcome: Open, Strategy: "default", Reason: "no strategy decided"}
}

// Before returns a new chain with s asked ahead of every strategy in c.
func (c Chain) Before(s ...Strategy) Chain {
	return append(append(Chain{}, s...), c...)
}

// After returns a new chain with s asked once every strategy in c has passed.
func (c Chain) After(s ...Strategy) Chain {
	return append(append(Chain{}, c...), s...)
}
