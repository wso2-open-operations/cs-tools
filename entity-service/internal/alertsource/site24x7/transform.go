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
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter.
const source = "Site24x7"

// tagList maps a canonical field to the TAGS prefix that carries it, such as "Service:checkout".
var tagList = map[string]string{
	"Service":     "Service",
	"Environment": "Environment",
	"Category":    "Category",
}

// Built-in defaults for a field the payload does not carry; an empty default leaves the field empty.
const (
	defaultService     = ""
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Production"
	defaultSeverity    = "Critical"
)

// severityMap maps Site24x7's monitor status to the canonical severity labels.
var severityMap = map[string]string{
	"DOWN":     "Critical",
	"CRITICAL": "Major",
	"TROUBLE":  "Minor",
	"UP":       "OK",
}

// ErrMissingBody is returned when the webhook is called with no body, or a body that isn't valid JSON at all.
var ErrMissingBody = errors.New("missing or invalid request body")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Transform parses a raw inbound Site24x7 alert and produces the canonical Alert, filling gaps with the built-in defaults.
func Transform(raw []byte) (Alert, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return Alert{}, ErrMissingBody
	}

	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrMissingBody, err)
	}
	if payload == nil {
		return Alert{}, fmt.Errorf("%w: body is not a JSON object", ErrMissingBody)
	}

	extracted := extractTags(strSlice(payload, "TAGS"), tagList)

	var d model.Defaults
	alert := Alert{
		Service:    d.Or("service", extracted["Service"], defaultService),
		MetricName: fieldvalue.Str(payload, "MONITORNAME"),
		// A missing or unrecognised STATUS takes the default severity rather than the raw status.
		Severity:         d.Or("severity", severityMap[fieldvalue.Str(payload, "STATUS")], defaultSeverity),
		Category:         d.Or("category", extracted["Category"], defaultCategory),
		Environment:      d.Or("environment", extracted["Environment"], defaultEnvironment),
		Source:           source,
		UniqueIdentifier: fieldvalue.Str(payload, "MONITOR_ID"),
		// Site24x7's reason text, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.Str(payload, "INCIDENT_REASON"),
	}
	alert.Defaulted = d.List()
	return alert, nil
}

// extractTags scans each tag for a tagList prefix; a tag with more than one colon loses everything after the second segment, matching the reference script's quirk.
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
