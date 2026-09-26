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

package dto

import "strconv"

// deploymentTypeIDs mirrors entity-service's private deploymentTypeToKey
// table (internal/service/sn_deployment_service.go) — the ServiceNow
// choice-list integer key for each DeploymentType enum value. The frontend
// was built against the old Ballerina backend, which forwarded this raw
// numeric key (deploymentTypeKey/typeKey) directly; entity-service's own
// contract is the plain string enum, so this backend translates both ways,
// same pattern as case_enum_mapping.go.
var deploymentTypeIDs = map[string]int{
	"development":        1,
	"qa":                 2,
	"staging":            3,
	"stress":             4,
	"uat":                5,
	"primary_production": 6,
}

// deploymentTypeIDStrings is deploymentTypeIDs with its values stringified --
// normalizeChoices (case_enum_mapping.go) takes a map[string]string, matching
// every other domainToID table it's called with (caseStateIDs,
// caseSeverityIDs, ...); deploymentTypeIDs stays map[string]int since
// deploymentTypeIDToEnum's reverse lookup and deploymentTypeIDToEnumPtr both
// need the numeric form.
var deploymentTypeIDStrings = func() map[string]string {
	m := make(map[string]string, len(deploymentTypeIDs))
	for enum, id := range deploymentTypeIDs {
		m[enum] = strconv.Itoa(id)
	}
	return m
}()

var deploymentTypeIDToEnum = func() map[int]string {
	m := make(map[int]string, len(deploymentTypeIDs))
	for enum, id := range deploymentTypeIDs {
		m[id] = enum
	}
	return m
}()

// deploymentTypeLabels are portal-owned display labels for each deployment
// type enum value.
var deploymentTypeLabels = map[string]string{
	"development":        "Development",
	"qa":                 "QA",
	"staging":            "Staging",
	"stress":             "Stress",
	"uat":                "UAT",
	"primary_production": "Primary Production",
}

// deploymentTypeRef converts entity-service's string enum into an
// {id, label} pair, id being the ServiceNow numeric key as a string.
func deploymentTypeRef(enum string) *IDLabelRef {
	if enum == "" {
		return nil
	}
	label := deploymentTypeLabels[enum]
	if label == "" {
		label = enum
	}
	id := ""
	if key, ok := deploymentTypeIDs[enum]; ok {
		id = strconv.Itoa(key)
	}
	return &IDLabelRef{ID: id, Label: label}
}

// normalizeDeploymentTypeChoices is normalizeCaseSeverityChoices for
// deployment types (see case_enum_mapping.go's normalizeChoices) --
// deployment_type_enum's Postgres labels ("DEVELOPMENT", "PRIMARY_PRODUCTION",
// etc.) are just the UPPER_SNAKE form of deploymentTypeIDs' own keys, so no
// enum-to-domain table is needed, same as case state.
//
// Without this, GET /projects/{id}/filters returned deploymentTypes.id as
// the raw Postgres label whenever entity-service ran in Postgres/dual-write
// mode, instead of the numeric ServiceNow-style id EditDeploymentModal.tsx's
// Number(form.typeKey) expects. Number("DEVELOPMENT") is NaN, and NaN is
// never equal to itself in JS, so the modal's "did the type actually change"
// check (newTypeKey !== originalTypeKey) was always true -- every save
// (even ones that didn't touch type) sent body.typeKey = NaN, which
// JSON.stringify turns into null, tripping the handler's
// "provide either detail fields or active, not both" guard.
func normalizeDeploymentTypeChoices(items []ReferenceItem) []ReferenceItem {
	return normalizeChoices(items, nil, deploymentTypeIDStrings, deploymentTypeLabels)
}

// deploymentTypeIDToEnumPtr translates a frontend-supplied numeric
// deployment-type key into entity-service's string enum, returning nil for
// an unrecognized id (never forwarded to entity-service as a raw number).
func deploymentTypeIDToEnumPtr(key int) *string {
	enum, ok := deploymentTypeIDToEnum[key]
	if !ok {
		return nil
	}
	return &enum
}
