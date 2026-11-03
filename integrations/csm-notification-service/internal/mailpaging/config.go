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

// Package mailpaging escalates an alert email by phone: csm-mail-paging-service
// publishes mail.alert.received for each matching email, and this package
// calls the people of the rule's first level at once, the next level after
// the level wait, and so on until every level has been called. It is
// separate from internal/paging, whose ladders are driven by cases and
// incidents; nothing here is shared with them but the Twilio client.
package mailpaging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	// DefaultLevelWait is how long a level has before the next one is called.
	DefaultLevelWait = 3 * time.Minute
	minLevelWait     = 10 * time.Second
	maxLevelWait     = time.Hour
)

// emailPattern is a sanity check on a configured address, not a validator.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Level is one rung: everyone on it is called together.
type Level struct {
	// Name is how the level is spoken and logged ("R1"); defaults to R<n>.
	Name string `json:"name"`
	// Emails are the people on the level. Their phone numbers are looked up in
	// entity-service when the level is called, not kept here.
	Emails []string `json:"emails"`
}

// Rule is the ladder for one mail rule (csm-mail-paging-service's
// MAIL_TRIGGER_RULES name).
type Rule struct {
	// LevelWait is a Go duration ("3m"); empty means DefaultLevelWait.
	LevelWait string  `json:"levelWait"`
	Levels    []Level `json:"levels"`

	wait time.Duration
}

// Wait is the resolved gap between two levels.
func (r Rule) Wait() time.Duration { return r.wait }

// Config maps a mail rule name to its ladder.
type Config struct {
	Rules map[string]Rule `json:"rules"`
}

// Load reads and validates the config file at path.
func Load(value string) (Config, error) {
	// The JSON itself may be set in the variable, like MAIL_TRIGGER_RULES, instead of a path to a mounted file.
	if v := strings.TrimSpace(value); strings.HasPrefix(v, "{") {
		return Parse([]byte(v))
	}
	raw, err := os.ReadFile(value) // #nosec G304 -- value is the operator-set MAIL_ESCALATION_CONFIG deployment env var, read once at startup, never request input
	if err != nil {
		return Config{}, fmt.Errorf("mailpaging: read %s: %w", value, err)
	}
	return Parse(raw)
}

// Parse decodes and validates a config document. Unknown keys are refused so
// a misspelt field cannot silently drop a level.
func Parse(raw []byte) (Config, error) {
	var c Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("mailpaging: parse config: %w", err)
	}
	if len(c.Rules) == 0 {
		return Config{}, errors.New("mailpaging: config has no rules")
	}
	for name, r := range c.Rules {
		if name == "" {
			return Config{}, errors.New("mailpaging: a rule has an empty name")
		}
		if len(r.Levels) == 0 {
			return Config{}, fmt.Errorf("mailpaging: rule %q has no levels", name)
		}
		r.wait = DefaultLevelWait
		if r.LevelWait != "" {
			d, err := time.ParseDuration(r.LevelWait)
			if err != nil || d < minLevelWait || d > maxLevelWait {
				return Config{}, fmt.Errorf("mailpaging: rule %q levelWait must be a duration between %s and %s", name, minLevelWait, maxLevelWait)
			}
			r.wait = d
		}
		seen := map[string]bool{}
		for i, l := range r.Levels {
			if l.Name == "" {
				l.Name = fmt.Sprintf("R%d", i+1)
				r.Levels[i].Name = l.Name
			}
			if seen[l.Name] {
				return Config{}, fmt.Errorf("mailpaging: rule %q repeats level %q", name, l.Name)
			}
			seen[l.Name] = true
			if len(l.Emails) == 0 {
				return Config{}, fmt.Errorf("mailpaging: rule %q level %q has no emails", name, l.Name)
			}
			uniq := make([]string, 0, len(l.Emails))
			have := map[string]bool{}
			for _, e := range l.Emails {
				e = strings.ToLower(strings.TrimSpace(e))
				if !emailPattern.MatchString(e) {
					return Config{}, fmt.Errorf("mailpaging: rule %q level %q has an entry that is not an email address", name, l.Name)
				}
				if !have[e] {
					have[e] = true
					uniq = append(uniq, e)
				}
			}
			r.Levels[i].Emails = uniq
		}
		c.Rules[name] = r
	}
	return c, nil
}
