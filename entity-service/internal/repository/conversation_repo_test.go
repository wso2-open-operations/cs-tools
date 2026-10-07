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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestConversationStateEnumRoundTrip locks in the one deliberate mismatch
// between domain.ConversationState and conversation_state_enum's real
// labels: "closed" is stored as 'CLOSE' (no D), not 'CLOSED'. Every other
// state matches by identity.
func TestConversationStateEnumRoundTrip(t *testing.T) {
	states := []domain.ConversationState{
		domain.ConversationStateActive,
		domain.ConversationStateResolved,
		domain.ConversationStateConverted,
		domain.ConversationStateAbandoned,
		domain.ConversationStateClosed,
	}
	for _, s := range states {
		enumValue := conversationStateToEnum(s)
		if got := conversationStateFromEnum(enumValue); got != s {
			t.Errorf("round trip failed for %q: toEnum=%q, fromEnum=%q", s, enumValue, got)
		}
	}

	if got := conversationStateToEnum(domain.ConversationStateClosed); got != "CLOSE" {
		t.Errorf("conversationStateToEnum(Closed) = %q, want %q", got, "CLOSE")
	}
	if got := conversationStateFromEnum("CLOSE"); got != domain.ConversationStateClosed {
		t.Errorf("conversationStateFromEnum(CLOSE) = %q, want %q", got, domain.ConversationStateClosed)
	}
	if got := conversationStateToEnum(domain.ConversationStateActive); got != "ACTIVE" {
		t.Errorf("conversationStateToEnum(Active) = %q, want %q", got, "ACTIVE")
	}
}

// TestConversationSearchQueries pins the shape of the two search statements:
// the COUNT touches only work_item and conversation (never the creator, project
// or linked-case joins, which cannot change a row count and are what made the
// old query cost grow with the number of matches), and the page query picks its
// rows first and resolves the creator per page row, once, by LATERAL ... LIMIT 1.
func TestConversationSearchQueries(t *testing.T) {
	where := "WHERE wi.type = 'CONVERSATION' AND wi.project_id = ANY($1::uuid[])"
	count, data := conversationSearchQueries(where, "wi.updated_on", "ASC", 1)

	if strings.Contains(count, `"user"`) || strings.Contains(count, "LEFT JOIN") {
		t.Errorf("count query must not carry display joins:\n%s", count)
	}
	if !strings.Contains(count, where) || !strings.HasPrefix(count, "SELECT COUNT(*)") {
		t.Errorf("count query lost its WHERE or is not a count:\n%s", count)
	}

	// The inner query picks the page: the WHERE, the caller's sort with the
	// wi.id tie-break, and LIMIT/OFFSET as the two placeholders after the
	// WHERE's own argument.
	for _, want := range []string{
		"FROM (SELECT wi.id",
		where,
		"ORDER BY wi.updated_on ASC, wi.id\n\t\t       LIMIT $2 OFFSET $3) page",
		"JOIN work_item wi ON wi.id = page.id",
		"LEFT JOIN LATERAL (",
		"ORDER BY u2.id\n\t\t     LIMIT 1",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("page query is missing %q:\n%s", want, data)
		}
	}
	// The same order is applied again to the joined rows, since a join does
	// not preserve the inner query's order.
	if got := strings.Count(data, "ORDER BY wi.updated_on ASC, wi.id"); got != 2 {
		t.Errorf("sort appears %d times in the page query, want 2 (inner page and outer result)", got)
	}
	// The creator join must not be a plain join on the non-unique email.
	if strings.Contains(data, `LEFT JOIN "user" u ON`) {
		t.Errorf("page query joins \"user\" directly instead of per page row:\n%s", data)
	}
	// Placeholders: one for the WHERE's argument, two for the page.
	if strings.Contains(data, "$4") {
		t.Errorf("page query uses more placeholders than the WHERE plus LIMIT/OFFSET:\n%s", data)
	}
}
