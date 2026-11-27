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

// Package severity reads alert severity labels and maps them to incident priority, category and contact type.
package severity

import (
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// severityRank orders the severity labels, 1 being the most severe; ok and clear resolve.
var severityRank = map[string]int{
	"critical": 1,
	"major":    2,
	"minor":    3,
	"warning":  4,
	"ok":       5,
	"clear":    0,
}

// Rank is a label's severity, with an unknown label treated as critical so a typo never pages less.
func Rank(label string) int {
	if n, ok := severityRank[strings.ToLower(strings.TrimSpace(label))]; ok {
		return n
	}
	return 1
}

// IsResolving reports whether a severity label says the problem is over.
func IsResolving(label string) bool {
	n := Rank(label)
	return n == 5 || n == 0
}

// ImpactUrgency is the incident priority a severity label asks for.
func ImpactUrgency(label string) (domain.IncidentImpact, domain.IncidentUrgency) {
	switch Rank(label) {
	case 2:
		return domain.IncidentImpactMedium, domain.IncidentUrgencyHigh
	case 3:
		return domain.IncidentImpactMedium, domain.IncidentUrgencyMedium
	case 4:
		return domain.IncidentImpactMedium, domain.IncidentUrgencyLow
	default:
		return domain.IncidentImpactHigh, domain.IncidentUrgencyHigh
	}
}

// IncidentCategory maps an alert category to an incident category, defaulting to a service interruption.
func IncidentCategory(category string) domain.IncidentCategory {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "security":
		return domain.IncidentCategorySecurity
	case "inquiry":
		return domain.IncidentCategoryInquiry
	default:
		return domain.IncidentCategoryServiceInterruption
	}
}

// ContactType is the incident contact type for sources CSM has one for, or nil.
func ContactType(source string) *domain.IncidentContactType {
	var ct domain.IncidentContactType
	switch strings.Map(keepAlphanumeric, strings.ToLower(source)) {
	case "azure", "azuremonitor":
		ct = domain.IncidentContactTypeAzure
	case "site24x7", "site247":
		ct = domain.IncidentContactTypeSite247
	case "sentinel", "azuresentinel", "microsoftsentinel":
		ct = domain.IncidentContactTypeSentinel
	default:
		return nil
	}
	return &ct
}

func keepAlphanumeric(r rune) rune {
	if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
		return r
	}
	return -1
}
