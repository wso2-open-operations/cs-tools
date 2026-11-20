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

package alertsource

import (
	"encoding/json"
	"strings"
)

// RIAFlag is a ria label or property: unset, or a request to send the alert to the RIA Creator or keep it on the core path.
type RIAFlag int8

// RIA flags; the zero value is unset, so a missing or unreadable label never changes routing.
const (
	RIAUnset RIAFlag = iota
	RIAOn
	RIAOff
)

// site24x7RIATagPrefix is the tag that carries the ria label, next to the Service: tag the transform reads.
const site24x7RIATagPrefix = "ria:"

// ParseRIAFlag reads true or false in any case and with surrounding spaces, or a JSON bool; anything else is unset.
func ParseRIAFlag(v any) RIAFlag {
	switch t := v.(type) {
	case bool:
		if t {
			return RIAOn
		}
		return RIAOff
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true":
			return RIAOn
		case "false":
			return RIAOff
		}
	}
	return RIAUnset
}

// RIALabels returns the ria label of each alert in a source body, in the order its transform returns them, or nil when the body is not JSON.
func RIALabels(source string, raw []byte) []RIAFlag {
	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	switch source {
	case "azure":
		return []RIAFlag{ParseRIAFlag(lookup(body, "data", "customProperties", "ria"))}
	case "gcp":
		return []RIAFlag{ParseRIAFlag(lookup(body, "incident", "policy_user_labels", "ria"))}
	case "icinga":
		return []RIAFlag{ParseRIAFlag(lookup(body, "vars", "ria"))}
	case "aws":
		return []RIAFlag{awsRIALabel(body)}
	case "site24x7":
		return []RIAFlag{site24x7RIALabel(body)}
	case "prometheus":
		return prometheusRIALabels(body)
	case "servicenow":
		if items, ok := body.([]any); ok {
			labels := make([]RIAFlag, len(items))
			for i, item := range items {
				labels[i] = ParseRIAFlag(lookup(item, "ria"))
			}
			return labels
		}
	}
	return []RIAFlag{ParseRIAFlag(lookup(body, "ria"))}
}

// awsRIALabel reads ria from the AlarmDescription JSON inside the SNS Message string, where the AWS transform reads service.
func awsRIALabel(body any) RIAFlag {
	message := decodeJSONString(lookup(body, "Message"))
	return ParseRIAFlag(lookup(decodeJSONString(lookup(message, "AlarmDescription")), "ria"))
}

// site24x7RIALabel reads the first RIA: tag, matched ignoring case like the Service: tag.
func site24x7RIALabel(body any) RIAFlag {
	tags, _ := lookup(body, "TAGS").([]any)
	for _, tag := range tags {
		s, ok := tag.(string)
		if ok && strings.HasPrefix(strings.ToLower(s), site24x7RIATagPrefix) {
			return ParseRIAFlag(s[len(site24x7RIATagPrefix):])
		}
	}
	return RIAUnset
}

// prometheusRIALabels reads each alert's labels.ria, skipping entries that are not objects exactly as the transform does.
func prometheusRIALabels(body any) []RIAFlag {
	alerts, _ := lookup(body, "alerts").([]any)
	labels := make([]RIAFlag, 0, len(alerts))
	for _, alert := range alerts {
		if _, ok := alert.(map[string]any); !ok {
			continue
		}
		labels = append(labels, ParseRIAFlag(lookup(alert, "labels", "ria")))
	}
	return labels
}

// lookup walks nested object keys and returns nil at the first missing key or non-object.
func lookup(v any, keys ...string) any {
	for _, key := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[key]
	}
	return v
}

// decodeJSONString parses a string that holds JSON, returning nil for anything else.
func decodeJSONString(v any) any {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(s), &decoded); err != nil {
		return nil
	}
	return decoded
}
