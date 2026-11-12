// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package muterule holds the active alert mute rules (one-time mute rules from CSM) and matches incidents against them.
package muterule

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"alert-core-service/internal/csm"
)

// Rule is one alert mute rule; every condition must match for an alert to be muted.
type Rule = csm.AlertMuteRule

// Fields are an alert's canonical fields an alert mute rule condition can name.
type Fields struct {
	Service          string
	Environment      string
	Source           string
	MetricName       string
	Severity         string
	Category         string
	UniqueIdentifier string
}

// value returns the field a condition names, and false for a field name it doesn't know.
func (f Fields) value(field string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "service":
		return f.Service, true
	case "environment":
		return f.Environment, true
	case "source":
		return f.Source, true
	case "metric_name":
		return f.MetricName, true
	case "severity":
		return f.Severity, true
	case "category":
		return f.Category, true
	case "unique_identifier":
		return f.UniqueIdentifier, true
	}
	return "", false
}

// environmentGroups maps each known environment spelling to its canonical group.
var environmentGroups = func() map[string]string {
	groups := map[string][]string{
		"production":  {"prod", "production", "prd", "live"},
		"staging":     {"stg", "stage", "staging"},
		"development": {"dev", "develop", "development"},
		"qa":          {"qa", "test", "testing"},
		"uat":         {"uat"},
		"stress":      {"stress", "perf", "performance"},
	}
	m := map[string]string{}
	for group, names := range groups {
		for _, n := range names {
			m[n] = group
		}
	}
	return m
}()

func normalize(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

// canonicalEnvironment is v's environment group, or v itself (lowercased, trimmed) when it is in none.
func canonicalEnvironment(v string) string {
	v = normalize(v)
	if g, ok := environmentGroups[v]; ok {
		return g
	}
	return v
}

// Matches reports whether every condition of b matches f: environment by canonical group, every other field by
// case-insensitive trimmed equality. A rule with no conditions, or naming an unknown field, matches nothing.
func Matches(b Rule, f Fields) bool {
	if len(b.Conditions) == 0 {
		return false
	}
	for _, c := range b.Conditions {
		got, ok := f.value(c.Field)
		if !ok {
			return false
		}
		if normalize(c.Field) == "environment" {
			if canonicalEnvironment(got) != canonicalEnvironment(c.Value) {
				return false
			}
		} else if normalize(got) != normalize(c.Value) {
			return false
		}
	}
	return true
}

// Fetch returns the alert mute rules active now.
type Fetch func(ctx context.Context) ([]Rule, error)

// Cache holds the last fetched active alert mute rules. A failed refresh keeps the last good set until it is older
// than maxStale; after that the cache counts as empty, so muting fails open.
type Cache struct {
	logger   *slog.Logger
	fetch    Fetch
	maxStale time.Duration
	now      func() time.Time

	mu        sync.RWMutex
	rules     []Rule
	fetchedAt time.Time
	// staleReported is set once the stale state has been logged at Error, so the
	// switch is logged once (and its recovery once), not on every failed refresh.
	staleReported bool
}

// Stale reports whether the cache matches nothing because its rules are missing or older than max staleness
// (muting has failed open), and how old they are (0 when none was ever fetched). /mutez reports it.
func (c *Cache) Stale() (stale bool, age time.Duration) {
	if c == nil {
		return false, 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.fetchedAt.IsZero() {
		return true, 0
	}
	age = c.now().Sub(c.fetchedAt)
	return age > c.maxStale, age
}

// NewCache returns an empty cache; call Refresh or Run to fill it.
func NewCache(logger *slog.Logger, fetch Fetch, maxStale time.Duration) *Cache {
	return &Cache{logger: logger, fetch: fetch, maxStale: maxStale, now: time.Now}
}

// Refresh replaces the cached rules with a fresh fetch; on error the previous set is kept.
func (c *Cache) Refresh(ctx context.Context) error {
	rules, err := c.fetch(ctx)
	if err != nil {
		c.mu.Lock()
		age := c.now().Sub(c.fetchedAt)
		stale := c.fetchedAt.IsZero() || age > c.maxStale
		report := stale && !c.staleReported
		if report {
			c.staleReported = true
		}
		c.mu.Unlock()
		switch {
		case report:
			// Muting has failed open: alerts a rule would mute create incidents and pages again. Error, once, so a
			// log-based alert can fire on it; /mutez reports the same state for a probe.
			c.logger.Error("alert mute rules: refresh failed and cache is stale, muting nothing until a refresh succeeds",
				"error", err, "max_staleness", c.maxStale.String(), "event", "alert_mute_rules_stale")
		case stale:
			c.logger.Warn("alert mute rules: refresh failed, cache still stale", "error", err)
		default:
			c.logger.Warn("alert mute rules: refresh failed, keeping last fetched rules", "error", err, "age", age.String())
		}
		return err
	}
	c.mu.Lock()
	changed := !sameIDs(c.rules, rules)
	recovered := c.staleReported
	c.rules, c.fetchedAt, c.staleReported = rules, c.now(), false
	c.mu.Unlock()
	if recovered {
		c.logger.Info("alert mute rules: refresh succeeded, muting again", "event", "alert_mute_rules_recovered")
	}
	if changed {
		numbers := make([]string, len(rules))
		for i, b := range rules {
			numbers[i] = b.Number
		}
		c.logger.Info("alert mute rules: active set changed", "count", len(rules), "rules", numbers)
	}
	return nil
}

func sameIDs(a, b []Rule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || !a[i].EndOn.Equal(b[i].EndOn) {
			return false
		}
	}
	return true
}

// Run refreshes every interval until ctx is done.
func (c *Cache) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rctx, cancel := context.WithTimeout(ctx, interval)
			_ = c.Refresh(rctx)
			cancel()
		}
	}
}

// Match returns the first cached rule active now whose conditions all match f. A nil cache, or one older than
// its max staleness, matches nothing.
func (c *Cache) Match(f Fields) (Rule, bool) {
	if c == nil {
		return Rule{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := c.now()
	if c.fetchedAt.IsZero() || now.Sub(c.fetchedAt) > c.maxStale {
		return Rule{}, false
	}
	for _, b := range c.rules {
		// The window is re-checked locally so a rule stops muting at its end, not at the next refresh.
		if now.Before(b.StartOn) || !now.Before(b.EndOn) {
			continue
		}
		if Matches(b, f) {
			return b, true
		}
	}
	return Rule{}, false
}
