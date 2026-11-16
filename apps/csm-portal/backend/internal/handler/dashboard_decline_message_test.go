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

package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
)

// TestDeclineMessage: the model's decline sentence is shown on one line,
// without control characters, and at most maxDeclineRunes long, cut at a
// word; one with nothing left is the refused message.
func TestDeclineMessage(t *testing.T) {
	// 501 runes; the limit falls inside the 60th "word".
	words := "x" + strings.Repeat(" word", 100)
	for _, tc := range []struct {
		name, reason, want string
	}{
		{"a sentence as it is", "Revenue is not in the portal.", "Revenue is not in the portal."},
		{"line breaks, tabs and runs of spaces", "Revenue is not\nin the portal.\r\n\n\tAsk  for cases by account.", "Revenue is not in the portal. Ask for cases by account."},
		{"control characters", "Revenue\x00 is not\x1b in the portal.", "Revenue is not in the portal."},
		{"padding", "  Revenue is not in the portal.  ", "Revenue is not in the portal."},
		{"empty", "", errMsgGenerateRefused},
		{"only spaces and control characters", " \n\t\x00 ", errMsgGenerateRefused},
		{"exactly the limit", strings.Repeat("a", maxDeclineRunes), strings.Repeat("a", maxDeclineRunes)},
		{"over the limit, cut at a word", words, "x" + strings.Repeat(" word", 59) + "…"},
		{"over the limit without a space", strings.Repeat("a", maxDeclineRunes+5), strings.Repeat("a", maxDeclineRunes) + "…"},
		{"the cut drops trailing punctuation", strings.Repeat("a", maxDeclineRunes-2) + ", and then more words follow here", strings.Repeat("a", maxDeclineRunes-2) + "…"},
		{"runes, not bytes", strings.Repeat("ශ", maxDeclineRunes), strings.Repeat("ශ", maxDeclineRunes)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := declineMessage(tc.reason)
			if got != tc.want {
				t.Errorf("declineMessage(%q) = %q, want %q", tc.reason, got, tc.want)
			}
			if n := utf8.RuneCountInString(got); n > maxDeclineRunes+1 {
				t.Errorf("%d runes, want at most %d", n, maxDeclineRunes+1)
			}
		})
	}
}

// TestServiceGenerator_DeclineIsShownOnOneLine: the generator hands the
// handler the decline as the chat shows it.
func TestServiceGenerator_DeclineIsShownOnOneLine(t *testing.T) {
	var declined *generateDeclinedError
	_, err := NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{Declined: "No revenue field.\nAsk for cases by account."}}, nil).
		Generate(context.Background(), "p", nil)
	if !errors.As(err, &declined) || declined.Reason != "No revenue field. Ask for cases by account." {
		t.Errorf("err = %v, want the decline on one line", err)
	}
}
