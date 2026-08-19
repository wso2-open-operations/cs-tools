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

// Package parity guards the Ballerina-to-Go response-field contract.
//
// The migration's dominant bug class is invisible: Ballerina's records are open
// (`json...;`) and loosely typed, while this service's sn* structs are closed by
// omission. A field ServiceNow sends that no Go struct declares is discarded by
// encoding/json with no error anywhere — nothing fails, a page just renders
// empty. That is how the Usage & Metrics page went blank (deployedProductCount),
// how ten project fields went missing, and how several case-detail fields were
// lost.
//
// This check turns that class into a build failure. It compares a frozen
// inventory of Ballerina's response fields against every json tag in this
// service, and fails on any field that is absent and not explicitly accounted
// for in known_gaps.json.
//
// The inventory is frozen on purpose: the Ballerina service lives in another
// repository and is being decommissioned, so the reference has to be captured
// while it still exists. Regenerate with extract_ballerina_fields.py against a
// digiops-cs checkout if it is still available.
package parity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type baselineFile struct {
	Source         string   `json:"source"`
	SourceRevision string   `json:"sourceRevision"`
	ResponseTypes  int      `json:"responseTypes"`
	Fields         []string `json:"fields"`
}

type knownGapsFile struct {
	Gaps map[string]string `json:"gaps"`
}

// jsonTag matches the name portion of a struct tag, e.g. `json:"createdOn,omitempty"`.
var jsonTag = regexp.MustCompile(`json:"([^",]+)`)

// decodeLayerGlob points at the ServiceNow-facing decode structs, relative to
// this package.
//
// Scoped to sn_*.go rather than the whole module on purpose, and it took a
// negative test to get right. Scanning every .go file made the check useless:
// removing json:"deployedProductCount" from the sn struct — the exact regression
// that blanked the Usage & Metrics page — did not fail, because domain/entity.go
// still declared the same name and a flat module-wide set counted it as
// understood. The silent drop happens precisely at this boundary, where the
// upstream payload is unmarshalled, so that is what has to be measured.
const decodeLayerGlob = "../service/sn_*.go"

// goJSONTagNames collects every json tag name declared anywhere in the service.
//
// Deliberately a flat set rather than per-struct: a field decoded on a sibling
// struct still proves the name is understood somewhere, and matching per-struct
// would produce constant false positives because the two services group fields
// differently (Ballerina nests account contacts inside ProjectResponse; this
// service has a separate ProjectAccountRef). Coarse and quiet beats precise and
// ignored.
func goJSONTagNames(t *testing.T) map[string]bool {
	t.Helper()
	paths, err := filepath.Glob(decodeLayerGlob)
	if err != nil {
		t.Fatalf("globbing %s: %v", decodeLayerGlob, err)
	}
	names := map[string]bool{}
	for _, path := range paths {
		// A field named only in a test is not implemented. Counting test files
		// would let a fixture keep a gap hidden.
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("reading %s: %v", path, readErr)
		}
		for _, m := range jsonTag.FindAllSubmatch(src, -1) {
			if n := strings.TrimSpace(string(m[1])); n != "" && n != "-" {
				names[n] = true
			}
		}
	}
	if len(names) < 100 {
		t.Fatalf("collected only %d json tags from %s; the glob is wrong and this check would pass vacuously",
			len(names), decodeLayerGlob)
	}
	return names
}

func loadJSON[T any](t *testing.T, path string) T {
	t.Helper()
	var out T
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return out
}

// timestampAlias maps Ballerina's `somethingAt` to this service's `somethingOn`.
// The `On` suffix for timestamps is a documented convention here (see CLAUDE.md),
// so closedAt/openedAt/resolvedAt are naming differences rather than missing
// data and must not be reported as gaps.
func timestampAlias(field string) string {
	if strings.HasSuffix(field, "At") {
		return strings.TrimSuffix(field, "At") + "On"
	}
	return field
}

// TestBallerinaResponseFieldParity fails when a Ballerina response field is
// decoded nowhere in this service and is not accounted for in known_gaps.json.
func TestBallerinaResponseFieldParity(t *testing.T) {
	baseline := loadJSON[baselineFile](t, "ballerina_response_fields.json")
	known := loadJSON[knownGapsFile](t, "known_gaps.json")
	goNames := goJSONTagNames(t)

	if len(baseline.Fields) == 0 {
		t.Fatal("baseline declares no fields; this check would pass vacuously")
	}

	var unaccounted []string
	for _, f := range baseline.Fields {
		if goNames[f] || goNames[timestampAlias(f)] {
			continue
		}
		if _, listed := known.Gaps[f]; listed {
			continue
		}
		unaccounted = append(unaccounted, f)
	}
	sort.Strings(unaccounted)

	if len(unaccounted) > 0 {
		t.Errorf(`%d Ballerina response field(s) are decoded nowhere in this service: %s

This is the silent-data-loss class: ServiceNow sends the field, no Go struct
declares it, encoding/json drops it without an error, and a page renders empty.

Either declare the field on the relevant sn* struct and map it onto the matching
domain view, or add it to known_gaps.json with a reason (tracked issue,
deliberate exclusion, or no consumer). Do not add it without one.

Baseline: %s @ %s`,
			len(unaccounted), strings.Join(unaccounted, ", "), baseline.Source, baseline.SourceRevision)
	}
}

// TestKnownGapsAreStillGaps stops the allowlist rotting. Once a field is
// implemented its entry must go, otherwise the list slowly becomes a permanent
// exemption nobody rechecks — and a future regression on that field would be
// silently tolerated.
func TestKnownGapsAreStillGaps(t *testing.T) {
	known := loadJSON[knownGapsFile](t, "known_gaps.json")
	goNames := goJSONTagNames(t)

	var stale []string
	for field := range known.Gaps {
		if goNames[field] || goNames[timestampAlias(field)] {
			stale = append(stale, field)
		}
	}
	sort.Strings(stale)

	if len(stale) > 0 {
		t.Errorf("%d entry/entries in known_gaps.json are now implemented and must be removed: %s",
			len(stale), strings.Join(stale, ", "))
	}
}

// TestKnownGapsHaveReasons keeps every exemption justified, so the list stays
// reviewable rather than becoming a dumping ground.
func TestKnownGapsHaveReasons(t *testing.T) {
	known := loadJSON[knownGapsFile](t, "known_gaps.json")
	if len(known.Gaps) == 0 {
		t.Skip("no known gaps recorded")
	}
	for field, reason := range known.Gaps {
		if len(strings.TrimSpace(reason)) < 20 {
			t.Errorf("known_gaps.json[%q] has no meaningful reason: %q", field, reason)
		}
	}
}

// TestBaselineIsPresentAndPlausible guards the frozen inventory itself: an empty
// or truncated baseline would make every other check here pass while testing
// nothing, the same trap as a gosec run that loads zero files.
func TestBaselineIsPresentAndPlausible(t *testing.T) {
	baseline := loadJSON[baselineFile](t, "ballerina_response_fields.json")
	if baseline.SourceRevision == "" || baseline.SourceRevision == "unknown" {
		t.Error("baseline has no source revision; it cannot be traced back to a Ballerina commit")
	}
	if got := len(baseline.Fields); got < 300 {
		t.Errorf("baseline has only %d fields; expected several hundred — regeneration likely truncated it", got)
	}
	if baseline.ResponseTypes < 100 {
		t.Errorf("baseline covers only %d response types; expected over a hundred", baseline.ResponseTypes)
	}
}

type criticalFieldsFile struct {
	Files map[string][]string `json:"files"`
}

// TestCriticalFieldsStayAtTheirDecodeSite closes the gap the module-wide check
// cannot cover.
//
// TestBallerinaResponseFieldParity compares a flat set of field names, so a field
// removed from one struct still looks present when another struct decodes the
// same name — deployedProductCount is decoded in both sn_deployment_service.go
// and sn_project_stats_service.go, so deleting either one alone slips through.
// That is not hypothetical: it is the exact field whose loss blanked the Usage &
// Metrics page.
//
// This pins the decode site itself for the fields already known to break a page
// when dropped. It is the guard that matters when resolving a merge conflict in
// these files: taking the wrong side deletes a tag, nothing fails to compile, no
// other test notices, and a screen silently renders empty.
func TestCriticalFieldsStayAtTheirDecodeSite(t *testing.T) {
	spec := loadJSON[criticalFieldsFile](t, "critical_fields.json")
	if len(spec.Files) == 0 {
		t.Fatal("critical_fields.json lists no files; this check would pass vacuously")
	}

	for path, fields := range spec.Files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: cannot read the file this guard covers: %v", path, err)
			continue
		}
		declared := map[string]bool{}
		for _, m := range jsonTag.FindAllSubmatch(src, -1) {
			declared[strings.TrimSpace(string(m[1]))] = true
		}
		var lost []string
		for _, f := range fields {
			if !declared[f] {
				lost = append(lost, f)
			}
		}
		sort.Strings(lost)
		if len(lost) > 0 {
			t.Errorf(`%s no longer decodes: %s

Each of these has already caused a blank or broken screen when dropped. If the
removal is deliberate, update critical_fields.json in the same change and say why
in the commit message. Do not edit that file merely to turn this build green.`,
				path, strings.Join(lost, ", "))
		}
	}
}
