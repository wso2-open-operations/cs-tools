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

import (
	"fmt"
	"strings"
)

// The label and title vocabulary the integration recognises, taken from the
// servicenow-integration repository that is installed in the product repos --
// .github/labels.yml and .github/workflows/issue_servicenow.yml.
//
// AN EARLIER VERSION OF THIS FILE HAD A DIFFERENT VOCABULARY ENTIRELY:
// Type/ChangeRequest, CRType/*, CRScope/*, state labels, impact and likelihood.
// Those came from GitHubIssueContentProcessor, a script include on the
// ServiceNow instance, and they appear in exactly one file across the product
// repos -- an issue template named "(DO NOT USE THIS YET) (WIP)". Nothing
// applies them, so nothing would ever have matched. The vocabulary below is
// the one the live workflows use.
//
// WHAT IS DELIBERATELY ABSENT, because the live integration has no equivalent:
// scope, impact, likelihood, and state labels. It never changes a record's
// state from GitHub at all -- its only writes are creating a case, editing a
// case's fields, and adding a comment. State travels the other way.
type GithubLabels struct {
	// TypeIncident and TypeServiceRequest identify the two templated kinds.
	TypeIncident       string
	TypeServiceRequest string

	// ClassByLabel maps a CR class label onto the service request's sr_type.
	//
	// NOT onto a change request. A [CR]: issue creates a SERVICE REQUEST --
	// issue_servicenow.yml sets case_type "Service Request", catalog
	// "Generic Requests", and carries the class as sr_type. The change request
	// proper is raised later, by a person, through the portal, with the
	// approval path and planned window an issue cannot supply.
	ClassByLabel map[string]string

	// StatusAssigned is written by the outbound sync when the case gains an
	// assignee. Listed here because the inbound half must not treat it as a
	// signal: it is our own output coming back.
	StatusAssigned string

	// ValidationPassed and ValidationFailed are applied by the repository's own
	// validation workflow, before this service ever sees the issue.
	ValidationPassed string
	ValidationFailed string
}

// Title prefixes. A change request carries no template label; the prefix is
// the only signal, which is why these are part of the vocabulary rather than
// an implementation detail of the matcher.
const (
	TitlePrefixChangeRequest          = "[CR]:"
	TitlePrefixEmergencyChangeRequest = "[ECR]:"
)

// DefaultGithubLabels is .github/labels.yml, verbatim.
func DefaultGithubLabels() GithubLabels {
	return GithubLabels{
		TypeIncident:       "Type/Incident",
		TypeServiceRequest: "Type/ServiceRequest",
		ClassByLabel: map[string]string{
			"CR/NormalChange":    "Normal Change",
			"CR/StandardChange":  "Standard Change",
			"CR/EmergencyChange": "Emergency Change",
		},
		StatusAssigned:   "Status/Assigned",
		ValidationPassed: "validation-passed",
		ValidationFailed: "validation-failed",
	}
}

// GithubLabelOverrides are the flat single-line strings Choreo can carry.
// Empty keeps the default.
type GithubLabelOverrides struct {
	TypeIncident       string // GITHUB_LABEL_TYPE_INCIDENT
	TypeServiceRequest string // GITHUB_LABEL_TYPE_SERVICE_REQUEST
	Class              string // GITHUB_LABELS_CLASS  "CR/NormalChange:Normal Change,..."
	StatusAssigned     string // GITHUB_LABEL_STATUS_ASSIGNED
}

// NewGithubLabels applies overrides to the defaults.
//
// An unparseable override is an ERROR, not a fallback. A label vocabulary that
// silently reverts to the default is a sync that recognises nothing and
// reports nothing -- refusing to start is the louder failure, and the cheaper
// one to diagnose.
func NewGithubLabels(o GithubLabelOverrides) (GithubLabels, error) {
	l := DefaultGithubLabels()

	if v := strings.TrimSpace(o.TypeIncident); v != "" {
		l.TypeIncident = v
	}
	if v := strings.TrimSpace(o.TypeServiceRequest); v != "" {
		l.TypeServiceRequest = v
	}
	if v := strings.TrimSpace(o.StatusAssigned); v != "" {
		l.StatusAssigned = v
	}
	if v := strings.TrimSpace(o.Class); v != "" {
		m, err := parseLabelMap(v, "GITHUB_LABELS_CLASS")
		if err == nil {
			err = validateClassValues(m)
		}
		if err != nil {
			return GithubLabels{}, err
		}
		l.ClassByLabel = m
	}
	return l, nil
}


// canonicalClassValues are the only values a class label may map to. A class
// value becomes the service request's sr_type and is written to u_sr_type, so
// an arbitrary replacement is not merely unusual -- it puts a value downstream
// consumers have never seen into the record.
//
// The labels themselves stay free-form, because a repository may well name them
// differently; what they resolve TO is fixed vocabulary. Checked at startup so
// a typo fails the deployment rather than silently classifying every change
// request as unrecognised and skipping it.
var canonicalClassValues = map[string]bool{
	"Normal Change":    true,
	"Standard Change":  true,
	"Emergency Change": true,
}

// validateClassValues rejects a GITHUB_LABELS_CLASS override that maps a label
// to something outside the canonical set.
func validateClassValues(m map[string]string) error {
	for label, value := range m {
		if !canonicalClassValues[value] {
			return fmt.Errorf(
				"GITHUB_LABELS_CLASS: %q maps to %q, which is not one of "+
					"\"Normal Change\", \"Standard Change\" or \"Emergency Change\"", label, value)
		}
	}
	return nil
}

// parseLabelMap reads "label:value,label:value". Values may contain spaces
// ("Normal Change"); labels may contain a slash ("CR/NormalChange").
func parseLabelMap(raw, name string) (map[string]string, error) {
	out := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		label, value, ok := strings.Cut(pair, ":")
		label, value = strings.TrimSpace(label), strings.TrimSpace(value)
		if !ok || label == "" || value == "" {
			return nil, fmt.Errorf("%s: %q is not label:value", name, pair)
		}
		out[label] = value
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no label:value pairs found in %q", name, raw)
	}
	return out, nil
}

// IsChangeRequestTitle reports whether a title marks the issue as a change
// request. Both prefixes count: [ECR]: is an emergency change, which is still
// a change request, and the class label is what distinguishes them.
func IsChangeRequestTitle(title string) bool {
	t := strings.TrimSpace(title)
	return strings.HasPrefix(t, TitlePrefixChangeRequest) ||
		strings.HasPrefix(t, TitlePrefixEmergencyChangeRequest)
}

// ClassOf returns the sr_type for an issue's labels, and whether
// exactly one was found. Zero or several is not a class we can act on: the
// record has one type, and guessing which would be worse than declining.
func (l GithubLabels) ClassOf(labels []string) (string, bool) {
	var found string
	for _, name := range labels {
		if v, ok := l.ClassByLabel[name]; ok {
			if found != "" {
				return "", false
			}
			found = v
		}
	}
	return found, found != ""
}

// The two catalogs issue_servicenow.yml routes to.
const (
	CatalogGenericRequests = "Generic Requests"
	CatalogGeneralRequests = "General Requests"
)

// ExtractTemplateFields reads a template-filled issue body into the u_-prefixed
// keys the service request stores in json_data.
//
// A SCAN, NOT THE REGEX extractFields.js USES. That one ends each section with
// a lookahead for the next "###", and Go's RE2 has no lookahead -- copying it
// across compiles at init and panics the process on startup. Walking the lines
// says the same thing and is easier to follow besides.
//
// The naming follows extractFields.js: lower-cased, punctuation to
// underscores, u_ in front. CS0441366 holds {"u_request_details": ...}, so
// matching it keeps a record raised from GitHub indistinguishable from one
// raised any other way.
//
// A body with no "###" sections yields nothing rather than guessing -- an
// issue written free-hand has no fields to capture.
func ExtractTemplateFields(body string) map[string]string {
	out := map[string]string{}
	var label string
	var value []string

	flush := func() {
		if label == "" {
			return
		}
		v := strings.TrimSpace(strings.Join(value, "\n"))
		// GitHub writes this for a field the author left blank.
		if v != "" && v != "_No response_" {
			out["u_"+templateKey(label)] = v
		}
		label, value = "", nil
	}

	for _, line := range strings.Split(body, "\n") {
		if h := strings.TrimSpace(line); strings.HasPrefix(h, "### ") {
			flush()
			label = strings.TrimSpace(strings.TrimPrefix(h, "### "))
			continue
		}
		if label != "" {
			value = append(value, line)
		}
	}
	flush()
	return out
}

func templateKey(label string) string {
	var b strings.Builder
	lastUnderscore := true
	for _, r := range strings.ToLower(label) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case !lastUnderscore:
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}
