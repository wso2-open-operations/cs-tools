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

package outagecommtask

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagecomm"
)

func TestRenderBody_EscapesMarkupAndNonASCII(t *testing.T) {
	out := renderBody(outagecomm.Decision{Body: "Impact — users in Zürich\n<script>"})
	for _, want := range []string{"Impact &#8212; users in Z&#252;rich", "<br>", "&lt;script&gt;"} {
		if !strings.Contains(out, want) {
			t.Errorf("body should contain %q, got %q", want, out)
		}
	}
	for _, r := range out {
		if r > 127 {
			t.Fatalf("rendered body must be pure ASCII, found %q", r)
		}
	}
}

type fakeSweeper struct {
	res   outagecomm.SweepResult
	err   error
	calls int
}

func (f *fakeSweeper) Sweep(context.Context, int) (outagecomm.SweepResult, error) {
	f.calls++
	return f.res, f.err
}

type fakeEmail struct {
	sends int
	fail  map[string]bool
}

func (f *fakeEmail) SendEmail(_ context.Context, _, _ []string, subject, _ string) error {
	f.sends++
	if f.fail[subject] {
		return errors.New("rejected")
	}
	return nil
}

func TestSendCommunications_NoRecipientsDoesNotSweep(t *testing.T) {
	s := &fakeSweeper{}
	if err := SendCommunications(s, &fakeEmail{}, nil, nil, true)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.calls != 0 {
		t.Fatal("with nowhere to deliver, the sweep must not run (it records decisions as sent)")
	}
}

func TestSendCommunications_OneFailureDoesNotStopTheRest(t *testing.T) {
	s := &fakeSweeper{res: outagecomm.SweepResult{Decisions: []outagecomm.Decision{
		{Number: "OUT1", Kind: "DECLARED", Subject: "s1"},
		{Number: "OUT2", Kind: "DECLARED", Subject: "s2"},
	}}}
	m := &fakeEmail{fail: map[string]bool{"s1": true}}
	err := SendCommunications(s, m, []string{"sre@example.com"}, nil, true)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "OUT1") {
		t.Fatalf("the failed decision must be reported, got %v", err)
	}
	if m.sends != 2 {
		t.Fatalf("every decision must be attempted, got %d sends", m.sends)
	}
}
