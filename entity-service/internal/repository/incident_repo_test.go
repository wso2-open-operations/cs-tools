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

package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestIncidentWhereClause_CorrelationIDExactMatch(t *testing.T) {
	id := "fp:abc123:169000"
	where, args := incidentWhereClause(domain.SearchIncidentsFilters{CorrelationID: &id}, nil, nil, nil, nil, nil, nil, nil, nil)

	if !strings.Contains(where, "inc.correlation_id = $1") {
		t.Fatalf("expected an exact-match correlation_id clause, got: %s", where)
	}
	if len(args) != 1 || args[0] != id {
		t.Fatalf("expected args [%q], got %v", id, args)
	}
}

func TestIncidentWhereClause_CorrelationIDOmittedWhenAbsentOrEmpty(t *testing.T) {
	for _, id := range []*string{nil, strPtr("")} {
		where, args := incidentWhereClause(domain.SearchIncidentsFilters{CorrelationID: id}, nil, nil, nil, nil, nil, nil, nil, nil)
		if strings.Contains(where, "correlation_id") {
			t.Fatalf("expected no correlation_id clause for %v, got: %s", id, where)
		}
		if len(args) != 0 {
			t.Fatalf("expected no args for %v, got %v", id, args)
		}
	}
}

// TestIncidentRepo_CreateIncidentComment_RejectsActivityType locks in
// CreateIncidentComment's guard against CommentTypeActivity -- mirroring
// CaseRepository.CreateCaseComment's identical restriction (case_repo.go):
// "activity" comments only ever arise from an upstream audit trail, never a
// caller-authored write. The guard runs before any query, so this is
// exercisable against a repo with a nil pool -- a real DB round trip would
// panic on a nil *pgxpool.Pool, which is exactly the point: reaching that
// far here would itself be the bug.
func TestIncidentRepo_CreateIncidentComment_RejectsActivityType(t *testing.T) {
	r := &incidentRepo{}
	_, err := r.CreateIncidentComment(context.Background(), "11111111-1111-1111-1111-111111111111", domain.CommentTypeActivity, "some content", "jane.doe@example.com")
	var ve *apierror.ValidationError
	if !errorsAsValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError for type=activity, got %T: %v", err, err)
	}
}

// TestIncidentRepo_CreateIncidentComment_RejectsUnknownType covers the other
// guard: any domain.CommentType value with no caseCommentTypeEnum mapping
// (there are only three: WORK_NOTE/COMMENT/APPROVAL_HISTORY) is rejected the
// same way, before any query runs.
func TestIncidentRepo_CreateIncidentComment_RejectsUnknownType(t *testing.T) {
	r := &incidentRepo{}
	_, err := r.CreateIncidentComment(context.Background(), "11111111-1111-1111-1111-111111111111", domain.CommentType("not_a_real_type"), "some content", "jane.doe@example.com")
	var ve *apierror.ValidationError
	if !errorsAsValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError for an unknown type, got %T: %v", err, err)
	}
}

// TestIncidentRepo_CreateIncidentComment_TypeMapping locks in the two
// writable comment types' Postgres enum mapping, reusing
// caseCommentTypeEnum/caseCommentEnumType (case_repo.go) -- the comment
// table and its comment_type_enum are shared across every work_item type,
// not case-specific despite the map's name, so incident intentionally
// reuses it rather than duplicating an identical map.
func TestIncidentRepo_CreateIncidentComment_TypeMapping(t *testing.T) {
	tests := []struct {
		commentType domain.CommentType
		wantEnum    string
	}{
		{domain.CommentTypeWorkNote, "WORK_NOTE"},
		{domain.CommentTypeComment, "COMMENT"},
	}
	for _, tt := range tests {
		gotEnum, ok := caseCommentTypeEnum[tt.commentType]
		if !ok || gotEnum != tt.wantEnum {
			t.Errorf("caseCommentTypeEnum[%q] = (%q, %v), want (%q, true)", tt.commentType, gotEnum, ok, tt.wantEnum)
		}
		if got := caseCommentEnumType[tt.wantEnum]; got != tt.commentType {
			t.Errorf("caseCommentEnumType[%q] = %q, want %q", tt.wantEnum, got, tt.commentType)
		}
	}
}

// TestIncidentReadMappings_ReturnAPISpellings locks in the read-side enum
// mappings: an incident read from Postgres carries the same state and
// resolution code values the API accepts and the ServiceNow data source
// returns. The webapp keys its state transitions by "CANCELLED", so a
// 'CANCELED' read back unmapped crashed the incident detail page.
func TestIncidentReadMappings_ReturnAPISpellings(t *testing.T) {
	str := func(s string) *string { return &s }
	deref := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}

	for label, want := range map[string]string{
		"CANCELED":    "CANCELLED",
		"NEW":         "NEW",
		"IN_PROGRESS": "IN_PROGRESS",
		"CLOSED":      "CLOSED",
	} {
		if got := deref(incidentStateFromEnum(str(label))); got != want {
			t.Errorf("incidentStateFromEnum(%q) = %q, want %q", label, got, want)
		}
	}
	if got := incidentStateFromEnum(nil); got != nil {
		t.Errorf("incidentStateFromEnum(nil) = %q, want nil", *got)
	}

	for label, want := range map[string]string{
		"SOLVED_WORK_AROUND":   "SOLVED_WORKAROUND",
		"NOT_ACTIONABLE_ALERT": "NOT_ACTIONABLE",
		"SOLVED_PERMANENTLY":   "SOLVED_PERMANENTLY",
		"DUPLICATE":            "DUPLICATE",
	} {
		if got := deref(incidentResolutionCodeFromEnum(str(label))); got != want {
			t.Errorf("incidentResolutionCodeFromEnum(%q) = %q, want %q", label, got, want)
		}
	}
	if got := incidentResolutionCodeFromEnum(nil); got != nil {
		t.Errorf("incidentResolutionCodeFromEnum(nil) = %q, want nil", *got)
	}
}

// errorsAsValidationError is a tiny local helper -- this package has no
// shared asValidationError helper the way internal/service does.
func errorsAsValidationError(err error, target **apierror.ValidationError) bool {
	ve, ok := err.(*apierror.ValidationError)
	if !ok {
		return false
	}
	*target = ve
	return true
}

func TestIncidentWhereClause_SLAFilters(t *testing.T) {
	min, max := 75, 100
	where, args := incidentWhereClause(domain.SearchIncidentsFilters{ParsedTaskSLA: &domain.TaskSLAFilter{MinBusinessElapsedPercent: &min, MaxBusinessElapsedPercent: &max}},
		nil, nil, nil, nil, nil, boolPtr(true), nil, nil)

	for _, want := range []string{
		"AND EXISTS (SELECT 1 FROM sla_live bs WHERE bs.work_item_id = wi.id AND bs.is_active IS TRUE AND bs.live_has_breached IS TRUE)",
		"AND EXISTS (SELECT 1 FROM sla_live tsla WHERE tsla.work_item_id = wi.id AND tsla.is_active IS TRUE AND tsla.live_elapsed_percentage >= $1::numeric AND tsla.live_elapsed_percentage <= $2::numeric)",
	} {
		if !strings.Contains(where, want) {
			t.Errorf("where %q missing %q", where, want)
		}
	}
	if len(args) != 2 || args[0] != 75 || args[1] != 100 {
		t.Errorf("args = %v, want [75 100]", args)
	}

	where, _ = incidentWhereClause(domain.SearchIncidentsFilters{}, nil, nil, nil, nil, nil, boolPtr(false), nil, nil)
	if !strings.Contains(where, "AND NOT EXISTS (SELECT 1 FROM sla_live bs") {
		t.Errorf("slaViolated=false should emit NOT EXISTS, got %q", where)
	}
}

// The shared predicates must produce exactly what the case search used to
// inline, with the work-item id expression the only variable.
func TestSLAPredicates_WorkItemIDIsParameter(t *testing.T) {
	if got, want := slaBreachedPredicate("x.id", false), " AND NOT EXISTS (SELECT 1 FROM sla_live bs WHERE bs.work_item_id = x.id AND bs.is_active IS TRUE AND bs.live_has_breached IS TRUE)"; got != want {
		t.Errorf("breached = %q, want %q", got, want)
	}
	zero := 0
	clause, args, next := slaPercentPredicate("x.id", &domain.TaskSLAFilter{MaxBusinessElapsedPercent: &zero}, []any{"a"}, 2)
	if clause != " AND EXISTS (SELECT 1 FROM sla_live tsla WHERE tsla.work_item_id = x.id AND tsla.is_active IS TRUE AND tsla.live_elapsed_percentage <= $2::numeric)" || len(args) != 2 || next != 3 {
		t.Errorf("percent = %q %v %d", clause, args, next)
	}
	if clause, _, next := slaPercentPredicate("x.id", nil, nil, 5); clause != "" || next != 5 {
		t.Errorf("nil filter should be a no-op, got %q %d", clause, next)
	}
}
