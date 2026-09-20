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
