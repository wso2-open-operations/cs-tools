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

package opencases

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
)

type fakeSearcher struct {
	calls int
	state string
}

func (f *fakeSearcher) SearchCasesInStateCreatedBeforeYesterday(_ context.Context, state string) ([]entitycases.Case, error) {
	f.calls++
	f.state = state
	return nil, nil
}

type fakeEmail struct{ sends int }

func (f *fakeEmail) SendEmail(context.Context, []string, []string, string, string) error {
	f.sends++
	return nil
}

type fakeGuard struct{ alreadySent bool }

func (g *fakeGuard) Once(ctx context.Context, send func(context.Context) error) error {
	if g.alreadySent {
		return nil
	}
	return send(ctx)
}

var to = []string{"reports@example.com"}

func TestSendReport_QueriesOpenStateAndSends(t *testing.T) {
	s, m := &fakeSearcher{}, &fakeEmail{}
	if err := SendReport(s, m, &fakeGuard{}, to, nil, true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.state != "open" || m.sends != 1 {
		t.Fatalf("expected one search for state open and one send, got state=%q sends=%d", s.state, m.sends)
	}
}

func TestSendReport_AlreadySentPeriodDoesNothing(t *testing.T) {
	s, m := &fakeSearcher{}, &fakeEmail{}
	if err := SendReport(s, m, &fakeGuard{alreadySent: true}, to, nil, true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.calls != 0 || m.sends != 0 {
		t.Fatalf("an already-sent period must neither query nor mail, got search=%d send=%d", s.calls, m.sends)
	}
}
