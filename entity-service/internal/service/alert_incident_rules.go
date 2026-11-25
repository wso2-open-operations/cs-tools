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

package service

// The rules alert-core (integrations/sre-alert-core-service) uses to turn an alert into a CSM incident, copied so an
// incident the alert processor raises looks exactly like one alert-core raised: severity, impact and urgency
// (internal/model), category, subject, contact type, correlation tag and creation note (internal/notify), and alert
// mute rule matching (internal/muterule). Keep the two copies in step until alert-core is retired.

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// alertSeverities maps a severity label to alert-core's 0-5 scale.
var alertSeverities = map[string]int{
	"critical": 1,
	"major":    2,
	"minor":    3,
	"warning":  4,
	"ok":       5,
	"clear":    0,
}

// alertSeverity is the label's level; an unknown label counts as critical (1), so a typo pages rather than hides.
func alertSeverity(label string) (level int, recognized bool) {
	if n, ok := alertSeverities[strings.ToLower(strings.TrimSpace(label))]; ok {
		return n, true
	}
	return 1, false
}

// alertIsResolving reports whether a severity level is a recovery signal (OK or Clear).
func alertIsResolving(level int) bool {
	return level == 5 || level == 0
}

// alertImpactUrgency maps a severity level to the incident's impact and urgency.
func alertImpactUrgency(level int) (domain.IncidentImpact, domain.IncidentUrgency) {
	switch level {
	case 2:
		return "MEDIUM", "HIGH"
	case 3:
		return "MEDIUM", "MEDIUM"
	case 4:
		return "MEDIUM", "LOW"
	case 5:
		return "LOW", "LOW"
	default:
		return "HIGH", "HIGH"
	}
}

// alertIncidentCategory is SECURITY or INQUIRY when the alert says so, else SERVICE_INTERRUPTION.
func alertIncidentCategory(category string) domain.IncidentCategory {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "security":
		return "SECURITY"
	case "inquiry":
		return "INQUIRY"
	default:
		return "SERVICE_INTERRUPTION"
	}
}

// alertIncidentSubject is the metric name, or the service when the alert has no metric name.
func alertIncidentSubject(a model.Alert) string {
	if a.MetricName != "" {
		return a.MetricName
	}
	return a.Service
}

// alertContactTypes maps a normalised source to its incident contact type; other sources send none.
var alertContactTypes = map[string]domain.IncidentContactType{
	"azure":             "AZURE",
	"azuremonitor":      "AZURE",
	"site24x7":          "SITE_247",
	"site247":           "SITE_247",
	"sentinel":          "SENTINEL",
	"azuresentinel":     "SENTINEL",
	"microsoftsentinel": "SENTINEL",
}

// alertContactType returns the contact type for an alert source, or nil.
func alertContactType(source string) *domain.IncidentContactType {
	var b strings.Builder
	for _, r := range strings.ToLower(source) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if ct, ok := alertContactTypes[b.String()]; ok {
		return &ct
	}
	return nil
}

// alertTagPrefix is the start of every correlation tag of a fingerprint, which finds its incidents.
func alertTagPrefix(fingerprint string) string {
	return "[fp:" + fingerprint[:min(len(fingerprint), 12)] + ":"
}

// alertCorrelationTag is alert-core's correlation id: the fingerprint's first 12 characters and the first alert's
// arrival in milliseconds. The same alert always gives the same tag, so a retried create finds the first one.
func alertCorrelationTag(fingerprint string, firstSeen time.Time) string {
	return fmt.Sprintf("%s%d]", alertTagPrefix(fingerprint), firstSeen.UnixMilli())
}

// alertCreationNote is the work note an incident is created with: the alert id and its fields as an HTML table.
func alertCreationNote(alertID string, a model.Alert) string {
	fields := [][2]string{
		{"service", a.Service},
		{"metric_name", a.MetricName},
		{"severity", a.Severity},
		{"category", a.Category},
		{"environment", a.Environment},
		{"source", a.Source},
		{"unique_identifier", a.UniqueIdentifier},
		{"description", a.Description},
	}
	var b strings.Builder
	b.WriteString(`<table style='border:1px solid #dcdcdc; border-collapse:collapse; width:100%; font-family:Arial, sans-serif; font-size:13px;'>`)
	for _, f := range fields {
		fmt.Fprintf(&b, `<tr><td style='background:#f5f5f5; font-weight:bold; width:30%%; padding:8px; border:1px solid #dcdcdc;'>%s</td>`+
			`<td style='padding:8px; border:1px solid #dcdcdc;'>%s</td></tr>`,
			html.EscapeString(f[0]), html.EscapeString(f[1]))
	}
	b.WriteString(`</table>`)
	return fmt.Sprintf("<p>Incident auto-created from Alert: %s</p>%s", html.EscapeString(alertID), b.String())
}

// alertEnvironmentGroups maps each known environment spelling to its group, so a rule on "prod" matches "production".
var alertEnvironmentGroups = func() map[string]string {
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

func normalizeMuteValue(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

func canonicalAlertEnvironment(v string) string {
	v = normalizeMuteValue(v)
	if g, ok := alertEnvironmentGroups[v]; ok {
		return g
	}
	return v
}

// alertMuteField returns the alert's value for a rule condition's field, and false for a field no alert has.
func alertMuteField(a model.Alert, field string) (string, bool) {
	switch normalizeMuteValue(field) {
	case "service":
		return a.Service, true
	case "environment":
		return a.Environment, true
	case "source":
		return a.Source, true
	case "metric_name":
		return a.MetricName, true
	case "severity":
		return a.Severity, true
	case "category":
		return a.Category, true
	case "unique_identifier":
		return a.UniqueIdentifier, true
	}
	return "", false
}

// alertMuteRuleMatches reports whether every condition of the rule matches the alert: environment by its group, every
// other field by trimmed, case-insensitive equality. A rule with no conditions, or naming an unknown field, matches nothing.
func alertMuteRuleMatches(rule domain.AlertMuteRule, a model.Alert) bool {
	if len(rule.Conditions) == 0 {
		return false
	}
	for _, c := range rule.Conditions {
		got, ok := alertMuteField(a, c.Field)
		if !ok {
			return false
		}
		if normalizeMuteValue(c.Field) == "environment" {
			if canonicalAlertEnvironment(got) != canonicalAlertEnvironment(c.Value) {
				return false
			}
		} else if normalizeMuteValue(got) != normalizeMuteValue(c.Value) {
			return false
		}
	}
	return true
}

// matchAlertMuteRule returns the first active rule that matches the alert at now.
func matchAlertMuteRule(rules []domain.AlertMuteRule, a model.Alert, now time.Time) (domain.AlertMuteRule, bool) {
	for _, rule := range rules {
		if rule.StateAt(now) == domain.AlertMuteRuleStateActive && alertMuteRuleMatches(rule, a) {
			return rule, true
		}
	}
	return domain.AlertMuteRule{}, false
}
