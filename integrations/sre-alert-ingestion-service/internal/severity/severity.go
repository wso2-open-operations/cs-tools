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

// Package severity maps a vendor-agnostic alert's severity/source fields
// onto CSM's own incident vocabulary (Impact/Urgency/ContactType). No
// upstream contract dictates these tables today — they are this service's
// own, deliberately conservative choice, documented here rather than
// scattered as magic strings.
package severity

import "strings"

// ImpactUrgency is a mapped Impact/Urgency pair for CreateIncidentRequest.
type ImpactUrgency struct {
	Impact  string
	Urgency string
}

// impactUrgencyTable maps an AlertRequest.Severity (case-insensitive) to the
// Impact/Urgency pair CSM's incident-creation contract expects.
//
// Chosen mapping, most to least severe:
//
//	critical -> HIGH   / HIGH
//	major    -> HIGH   / MEDIUM
//	minor    -> MEDIUM / MEDIUM
//	warning  -> LOW    / MEDIUM
//	ok       -> LOW    / LOW
//	<other>  -> LOW    / LOW   (unrecognized severity fails safe, not open)
//
// Urgency is kept at or above Impact for every tier except the two extremes
// (critical, ok) — a "minor" business impact from an alerting tool can still
// warrant timely engineer attention (e.g. a leading indicator before it
// becomes major), whereas a fully-resolved ("ok") signal warrants neither.
var impactUrgencyTable = map[string]ImpactUrgency{
	"critical": {Impact: "HIGH", Urgency: "HIGH"},
	"major":    {Impact: "HIGH", Urgency: "MEDIUM"},
	"minor":    {Impact: "MEDIUM", Urgency: "MEDIUM"},
	"warning":  {Impact: "LOW", Urgency: "MEDIUM"},
	"ok":       {Impact: "LOW", Urgency: "LOW"},
}

// defaultImpactUrgency is used for any severity value not present in
// impactUrgencyTable (including "ok"'s explicit entry, listed above for
// clarity even though it matches the default) — fails toward the least
// alarming classification for input this service doesn't recognize, rather
// than guessing high.
var defaultImpactUrgency = ImpactUrgency{Impact: "LOW", Urgency: "LOW"}

// MapImpactUrgency returns the Impact/Urgency pair for the given alert
// severity (matched case-insensitively; surrounding whitespace is trimmed).
func MapImpactUrgency(alertSeverity string) ImpactUrgency {
	key := strings.ToLower(strings.TrimSpace(alertSeverity))
	if iu, ok := impactUrgencyTable[key]; ok {
		return iu
	}
	return defaultImpactUrgency
}

// contactTypeTable maps an AlertRequest.Source (case-insensitive) to one of
// CreateIncidentRequest.ContactType's existing enum values. Only sources
// with a clear existing match are mapped; everything else omits
// ContactType entirely (see MapContactType) rather than guessing at a value
// this service was not told exists.
var contactTypeTable = map[string]string{
	"azure":              "AZURE",
	"site24x7":           "SITE_247",
	"site247":            "SITE_247",
	"sentinel":           "SENTINEL",
	"microsoft-sentinel": "SENTINEL",
}

// MapContactType returns the ContactType enum value for the given alert
// source, and false if no existing enum value fits — callers should omit
// ContactType from the request in that case (see CreateIncidentRequest's
// `omitempty` tag) rather than invent one.
func MapContactType(alertSource string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(alertSource))
	ct, ok := contactTypeTable[key]
	return ct, ok
}

// validCategories is the exact set CreateIncidentRequest.Category accepts.
var validCategories = map[string]string{
	"inquiry":              "INQUIRY",
	"service_interruption": "SERVICE_INTERRUPTION",
	"security":             "SECURITY",
}

// defaultCategory is used when AlertRequest.Category is empty or doesn't
// match one of CreateIncidentRequest's three accepted values. Every alert
// this service ingests originates from a monitoring/alerting tool signaling
// something is (or may be) actively wrong, which is what SERVICE_INTERRUPTION
// represents — a safer default than INQUIRY (implies no active problem) or
// SECURITY (a specific claim this service has no basis to make on a caller's
// behalf).
const defaultCategory = "SERVICE_INTERRUPTION"

// MapCategory returns alertCategory uppercased if it matches one of
// CreateIncidentRequest.Category's three accepted values (matched
// case-insensitively), else defaultCategory.
func MapCategory(alertCategory string) string {
	key := strings.ToLower(strings.TrimSpace(alertCategory))
	if c, ok := validCategories[key]; ok {
		return c
	}
	return defaultCategory
}
