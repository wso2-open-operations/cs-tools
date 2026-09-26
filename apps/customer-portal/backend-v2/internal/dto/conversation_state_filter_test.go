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

import (
	"errors"
	"testing"
)

// ServiceNow's conversation-state choice list carries "Open" as id 1 — a chat
// created but never answered by the assistant. Neither this backend nor
// entity-service mapped it, so it was skipped, which left States empty. An empty
// States is "no state filter" to entity-service rather than "no matches", so
// filtering by Open returned every conversation in the project (1072 of them on
// staging) while looking like a working filter. The same gap left 163
// conversations rendering with no state at all.

// TestBuildEntitySearchConversationsRequest_MapsOpenState covers the state that
// was missing: Open must translate, not be refused and not be dropped.
func TestBuildEntitySearchConversationsRequest_MapsOpenState(t *testing.T) {
	got, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{1}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Filters.States) != 1 || got.Filters.States[0] != "OPEN" {
		t.Fatalf("states = %v, want [OPEN]", got.Filters.States)
	}
}

// Open renders with its own label rather than falling back to the raw enum —
// these conversations previously showed no state at all.
func TestConversationStateRef_LabelsOpen(t *testing.T) {
	state := "OPEN"
	got := conversationStateRef(&state)
	if got == nil {
		t.Fatal("expected a state ref for OPEN")
	}
	if got.ID != "1" || got.Label != "Open" {
		t.Fatalf("ref = %+v, want {ID:1 Label:Open}", got)
	}
}

// A state the search cannot apply must not widen the result set, whatever its
// value — the Open id is the one users hit, but any unknown id did the same.
func TestBuildEntitySearchConversationsRequest_RejectsArbitraryUnknownState(t *testing.T) {
	_, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{99}},
	})
	if !errors.Is(err, ErrUnsupportedConversationState) {
		t.Fatalf("error = %v, want ErrUnsupportedConversationState", err)
	}
}

// One bad id among good ones must still fail: a partial translation would
// silently broaden the filter to the states it happened to understand.
func TestBuildEntitySearchConversationsRequest_RejectsPartiallyMappedStates(t *testing.T) {
	_, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{2, 98}},
	})
	if !errors.Is(err, ErrUnsupportedConversationState) {
		t.Fatalf("error = %v, want ErrUnsupportedConversationState", err)
	}
}

// The mapped states still work, and still translate to entity-service's enum
// vocabulary rather than the numeric ids the frontend sends.
func TestBuildEntitySearchConversationsRequest_MapsSupportedStates(t *testing.T) {
	got, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{2, 3}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Filters.States) != 2 || got.Filters.States[0] != "ACTIVE" || got.Filters.States[1] != "RESOLVED" {
		t.Fatalf("states = %v, want [ACTIVE RESOLVED]", got.Filters.States)
	}
	if len(got.Filters.ProjectIDs) != 1 || got.Filters.ProjectIDs[0] != "p-1" {
		t.Errorf("project scope must come from the path: %v", got.Filters.ProjectIDs)
	}
}

// No state filter at all is a legitimate request — "show me everything" is fine
// when the caller actually asked for everything.
func TestBuildEntitySearchConversationsRequest_NoStateFilterIsAllowed(t *testing.T) {
	got, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Filters.States) != 0 {
		t.Fatalf("states = %v, want empty", got.Filters.States)
	}
}

// Every state ServiceNow offers in the choice list must now be filterable.
// This is the invariant the original bug broke: a state present in the
// dropdown but absent from the map produced an unfiltered search.
func TestBuildEntitySearchConversationsRequest_EveryOfferedStateIsFilterable(t *testing.T) {
	// Exactly the ids /projects/{id}/filters returns.
	for _, id := range []int{1, 2, 3, 4, 5, 6} {
		got, err := BuildEntitySearchConversationsRequest("p-1", ConversationSearchRequest{
			Filters: ConversationSearchFilters{StateKeys: []int{id}},
		})
		if err != nil {
			t.Errorf("state id %d is offered in the filter list but cannot be filtered: %v", id, err)
			continue
		}
		if len(got.Filters.States) != 1 {
			t.Errorf("state id %d translated to %v, want exactly one state", id, got.Filters.States)
		}
	}
}
