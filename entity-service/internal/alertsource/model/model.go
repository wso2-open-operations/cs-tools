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

// Package model is the canonical alert stored in the alert table's alert column.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Alert's field names and JSON keys are the stored contract; do not rename them.
type Alert struct {
	Service          string `json:"service"`
	MetricName       string `json:"metric_name"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Environment      string `json:"environment"`
	Source           string `json:"source"`
	UniqueIdentifier string `json:"unique_identifier"`
	Description      string `json:"description"`
	// AssignmentGroup is the optional CSM assignment group the alert names for itself, such as an AWS alarm's "assignment_group".
	AssignmentGroup string `json:"assignment_group,omitempty"`
	// SourceTopic and SourceAccount optionally say where the alert came from, such as the AWS SNS TopicArn and account id.
	SourceTopic   string `json:"source_topic,omitempty"`
	SourceAccount string `json:"source_account,omitempty"`
	// Defaulted lists the fields the payload did not carry, which the transform filled with the source's default; it is not part of the fingerprint.
	Defaulted []Defaulted `json:"defaulted,omitempty"`
}

// Defaulted is one field the payload did not carry and the value the transform used instead.
type Defaulted struct {
	// Field is the canonical field name: service, metric_name, severity, category or environment.
	Field string `json:"field"`
	Value string `json:"value"`
}

// Defaults collects the fields a transform fills with a default; the zero value is ready to use.
type Defaults struct {
	used []Defaulted
}

// Or returns value when it is not empty; otherwise it returns def and, when def is not empty, notes field as defaulted to it.
func (d *Defaults) Or(field, value, def string) string {
	if value != "" {
		return value
	}
	if def != "" {
		d.used = append(d.used, Defaulted{Field: field, Value: def})
	}
	return def
}

// List returns the defaulted fields in the order they were filled, or nil when there are none.
func (d *Defaults) List() []Defaulted {
	return d.used
}

// Fingerprint identifies the alerts of one source, service, metric, environment and identifier; alerts with the same fingerprint fold into one incident.
func Fingerprint(a Alert) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{a.Source, a.Service, a.MetricName, a.Environment, a.UniqueIdentifier}, "|")))
	return hex.EncodeToString(sum[:])
}
