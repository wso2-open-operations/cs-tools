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

package stalecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
)

type fakeSearcher struct {
	calls int
	err   error
}

func (f *fakeSearcher) SearchOpenCasesOlderThan(context.Context, time.Duration) ([]entitycases.Case, error) {
	f.calls++
	return []entitycases.Case{{Number: "CS0001", CreatedOn: time.Now().Add(-40 * 24 * time.Hour)}}, f.err
}

type fakeEmail struct {
	sends int
	err   error
}

func (f *fakeEmail) SendEmail(context.Context, []string, []string, string, string) error {
	f.sends++
	return f.err
}

// fakeGuard runs send only when alreadySent is false, mimicking
// reportguard.Guard's skip on a period already recorded.
type fakeGuard struct {
	alreadySent bool
	calls       int
}

func (g *fakeGuard) Once(ctx context.Context, send func(context.Context) error) error {
	g.calls++
	if g.alreadySent {
		return nil
	}
	return send(ctx)
}

var to = []string{"reports@example.com"}

func TestSendReport_SendsThroughTheGuard(t *testing.T) {
	s, m, g := &fakeSearcher{}, &fakeEmail{}, &fakeGuard{}
	if err := SendReport(s, m, g, 30*24*time.Hour, to, nil, true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.calls != 1 || s.calls != 1 || m.sends != 1 {
		t.Fatalf("expected one guarded search and send, got guard=%d search=%d send=%d", g.calls, s.calls, m.sends)
	}
}

func TestSendReport_AlreadySentPeriodDoesNothing(t *testing.T) {
	s, m, g := &fakeSearcher{}, &fakeEmail{}, &fakeGuard{alreadySent: true}
	if err := SendReport(s, m, g, 30*24*time.Hour, to, nil, true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.calls != 0 || m.sends != 0 {
		t.Fatalf("an already-sent period must neither query nor mail, got search=%d send=%d", s.calls, m.sends)
	}
}

func TestSendReport_DisabledOrNoRecipientsSkipsEverything(t *testing.T) {
	for _, tc := range []struct {
		name    string
		to      []string
		enabled bool
	}{{"disabled", to, false}, {"no recipients", nil, true}} {
		s, m, g := &fakeSearcher{}, &fakeEmail{}, &fakeGuard{}
		if err := SendReport(s, m, g, time.Hour, tc.to, nil, tc.enabled)(context.Background()); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if g.calls+s.calls+m.sends != 0 {
			t.Fatalf("%s: expected no work at all, got guard=%d search=%d send=%d", tc.name, g.calls, s.calls, m.sends)
		}
	}
}

func TestSendReport_SendFailureIsReturned(t *testing.T) {
	m := &fakeEmail{err: errors.New("mail down")}
	if err := SendReport(&fakeSearcher{}, m, nil, time.Hour, to, nil, true)(context.Background()); err == nil {
		t.Fatal("a failed send must be returned")
	}
}
