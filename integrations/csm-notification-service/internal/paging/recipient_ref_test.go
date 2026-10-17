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

package paging

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRecipientLogID(t *testing.T) {
	withID := Recipient{UserID: " u-123 ", Name: "Jane Doe", Email: "jane@example.com"}
	if got := withID.LogID(); got != "u-123" {
		t.Fatalf("LogID with a user id = %q; want u-123", got)
	}

	byEmail := Recipient{Name: "Jane Doe", Email: "Jane@Example.com "}
	got := byEmail.LogID()
	if !strings.HasPrefix(got, "e:") || len(got) != len("e:")+12 {
		t.Fatalf("LogID from an email = %q; want e: and 12 hex digits", got)
	}
	if again := (Recipient{Email: "jane@example.com"}).LogID(); again != got {
		t.Fatalf("LogID is not stable across case and spaces: %q vs %q", again, got)
	}
	if other := (Recipient{Email: "john@example.com"}).LogID(); other == got {
		t.Fatalf("two emails share LogID %q", got)
	}
	for _, leak := range []string{"jane", "Jane", "example"} {
		if strings.Contains(got, leak) {
			t.Fatalf("LogID %q carries %q", got, leak)
		}
	}

	if got := (Recipient{Name: "Jane Doe"}).LogID(); got != "unknown" {
		t.Fatalf("LogID with neither id nor email = %q; want unknown", got)
	}
}

// Every log line this package writes about a person names them by LogID: the
// log channel's "would notify" line is the one that used to carry the name.
func TestLogNotifier_NamesTheRecipientByID(t *testing.T) {
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	call := PlannedCall{Level: Level0, Recipient: Recipient{
		UserID: "u-123", Name: "Jane Doe", Email: "jane@example.com", Phone: "+94770000001",
	}}
	if _, err := (logNotifier{}).Deliver(context.Background(), Plan{}, call); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "recipientId=u-123") {
		t.Fatalf("log line = %s; want recipientId=u-123", out)
	}
	for _, leak := range []string{"Jane", "jane@example.com", "0000001"} {
		if strings.Contains(out, leak) {
			t.Fatalf("log line carries %q: %s", leak, out)
		}
	}
}
