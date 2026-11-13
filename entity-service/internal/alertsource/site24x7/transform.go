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

// Package site24x7 transforms an inbound Site24x7 monitor alert into the canonical alert, a Go port of ServiceNow's Site24x7 Scripted REST Resource.
package site24x7

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter; unlike every other source, it is not overridable via config.
const source = "Site24x7"

// severityMap maps Site24x7's monitor status to the canonical severity labels.
var severityMap = map[string]string{
	"DOWN":     "Critical",
	"CRITICAL": "Major",
	"TROUBLE":  "Minor",
	"UP":       "OK",
}

// ErrMissingBody is returned when the webhook is called with no body, or a body that isn't valid JSON at all.
var ErrMissingBody = errors.New("missing or invalid request body")

// ErrMissingStatus is returned when neither the payload's STATUS field nor a configured Defaults.Severity is present.
var ErrMissingStatus = errors.New("missing required field STATUS")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Config is the operator-supplied "edge.api.site24x7.alert.config" system property: TagList maps a canonical field to its tag prefix, Defaults is the fallback value per field.
type Config struct {
	TagList  map[string]string `json:"TagList"`
	Defaults map[string]string `json:"Defaults"`
}

// LoadConfig reads the Config from the SITE24X7_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := fieldvalue.AlertConfigRaw("SITE24X7_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return Config{}, fmt.Errorf("invalid SITE24X7_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound Site24x7 alert and produces the canonical Alert, cfg supplying the tag-prefix mapping and defaults.
func Transform(raw []byte, cfg Config) (Alert, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return Alert{}, ErrMissingBody
	}

	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrMissingBody, err)
	}

	status := fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "STATUS"), cfg.Defaults["Severity"])
	if status == "" {
		return Alert{}, ErrMissingStatus
	}

	extracted := extractTags(strSlice(payload, "TAGS"), cfg.TagList)

	alert := Alert{
		Service:          fieldvalue.FirstNonEmpty(extracted["Service"], cfg.Defaults["Service"]),
		MetricName:       fieldvalue.Str(payload, "MONITORNAME"),
		Severity:         mapSeverity(status, cfg.Defaults["Severity"]),
		Category:         fieldvalue.FirstNonEmpty(extracted["Category"], cfg.Defaults["Category"]),
		Environment:      fieldvalue.FirstNonEmpty(extracted["Environment"], cfg.Defaults["Environment"]),
		Source:           source,
		UniqueIdentifier: fieldvalue.Str(payload, "MONITOR_ID"),
		// Site24x7's reason text, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.Str(payload, "INCIDENT_REASON"),
	}
	return alert, nil
}

// extractTags scans each tag for a configured prefix; a tag with more than one colon loses everything after the second segment, matching the reference script's quirk.
func extractTags(tags []string, tagList map[string]string) map[string]string {
	result := make(map[string]string)
	for _, tag := range tags {
		for field, prefix := range tagList {
			p := prefix + ":"
			if !strings.HasPrefix(tag, p) {
				continue
			}
			parts := strings.Split(tag, ":")
			if len(parts) > 1 {
				result[field] = strings.TrimSpace(parts[1])
			} else {
				result[field] = ""
			}
		}
	}
	return result
}

// mapSeverity maps a Site24x7 monitor status to a canonical severity, falling back to the configured default rather than the raw status.
func mapSeverity(status, defaultSeverity string) string {
	return fieldvalue.FirstNonEmpty(severityMap[status], defaultSeverity)
}

// strSlice reads a payload field expected to be a JSON array of strings.
func strSlice(payload map[string]any, key string) []string {
	raw, ok := payload[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
