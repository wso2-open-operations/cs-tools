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

// Package model is the canonical alert written to the alerts.alert column, field for field with sre-alert-core-service's model.Alert.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Alert's field names and JSON keys are fixed by the contract with alerts-core; do not add fields.
type Alert struct {
	Service          string `json:"service"`
	MetricName       string `json:"metric_name"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Environment      string `json:"environment"`
	Source           string `json:"source"`
	UniqueIdentifier string `json:"unique_identifier"`
	Description      string `json:"description"`
	// AssignmentGroup is a CSM assignment group the alert names for itself (an AWS alarm's
	// AlarmDescription "assignment_group"); the core resolves it to a group id. Optional.
	AssignmentGroup string `json:"assignment_group,omitempty"`
	// SourceTopic and SourceAccount say where the alert was sent from -- for AWS, the SNS
	// TopicArn and the AWS account id. The core maps them to an assignment group when the
	// alert names none and its service has no support group. Optional.
	SourceTopic   string `json:"source_topic,omitempty"`
	SourceAccount string `json:"source_account,omitempty"`
}

// Fingerprint must match sre-alert-core-service's model.Fingerprint; alerts-core uses it to claim one incident's alerts together.
func Fingerprint(a Alert) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{a.Source, a.Service, a.MetricName, a.Environment, a.UniqueIdentifier}, "|")))
	return hex.EncodeToString(sum[:])
}
