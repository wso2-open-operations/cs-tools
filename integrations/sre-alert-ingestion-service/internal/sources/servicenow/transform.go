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

// Package servicenow accepts alerts ServiceNow has already transformed into the eight canonical fields, a temporary route for the parallel run.
package servicenow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"sre-alert-ingestion-service/internal/model"
	"sre-alert-ingestion-service/internal/sources/jsonnum"
	"sre-alert-ingestion-service/utils"
)

// ErrNoAlert is returned for a body that holds no canonical alert.
var ErrNoAlert = errors.New("body must be a canonical alert object or a non-empty array of them")

// fields are the canonical JSON keys; an object must carry at least one.
var fields = []string{
	"service", "metric_name", "severity", "category",
	"environment", "source", "unique_identifier", "description",
}

// Transform parses one canonical alert object, or an array of them.
func Transform(raw []byte) ([]model.Alert, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '[' {
		var items []map[string]any
		if err := jsonnum.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNoAlert, err)
		}
		if len(items) == 0 {
			return nil, ErrNoAlert
		}
		out := make([]model.Alert, len(items))
		for i, m := range items {
			a, err := fromMap(m)
			if err != nil {
				return nil, fmt.Errorf("alert %d: %w", i, err)
			}
			out[i] = a
		}
		return out, nil
	}
	var m map[string]any
	if err := jsonnum.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoAlert, err)
	}
	a, err := fromMap(m)
	if err != nil {
		return nil, err
	}
	return []model.Alert{a}, nil
}

func fromMap(m map[string]any) (model.Alert, error) {
	if !hasCanonicalField(m) {
		return model.Alert{}, ErrNoAlert
	}
	return model.Alert{
		Service:          utils.Str(m, "service"),
		MetricName:       utils.Str(m, "metric_name"),
		Severity:         utils.Str(m, "severity"),
		Category:         utils.Str(m, "category"),
		Environment:      utils.Str(m, "environment"),
		Source:           utils.Str(m, "source"),
		UniqueIdentifier: utils.Str(m, "unique_identifier"),
		Description:      description(m["description"]),
	}, nil
}

func hasCanonicalField(m map[string]any) bool {
	for _, f := range fields {
		if _, ok := m[f]; ok {
			return true
		}
	}
	return false
}

// description keeps a string as sent; an object or array becomes compact JSON.
func description(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}
