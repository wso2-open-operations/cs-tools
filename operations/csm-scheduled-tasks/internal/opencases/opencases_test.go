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

package opencases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/opencases"
)

type fakeSearcher struct {
	cases []entitycases.Case
	err   error
}

func (f fakeSearcher) SearchCasesInStateCreatedBeforeYesterday(context.Context, string) ([]entitycases.Case, error) {
	return f.cases, f.err
}

type fakeSender struct {
	calls int
	body  string
}

func (f *fakeSender) SendEmail(_ context.Context, _, _ []string, _, body string) error {
	f.calls++
	f.body = body
	return nil
}

func TestSendReportSendsNothingWhenNoCaseMatches(t *testing.T) {
	email := &fakeSender{}
	h := opencases.SendReport(fakeSearcher{}, email, []string{"jane.doe@example.com"}, nil, "", true)
	if err := h(context.Background()); err != nil {
		t.Fatalf("an empty result must succeed: %v", err)
	}
	if email.calls != 0 {
		t.Errorf("an empty result must not send mail, sent %d", email.calls)
	}
}

func TestSendReportSendsWhenCasesMatch(t *testing.T) {
	email := &fakeSender{}
	found := []entitycases.Case{{Number: "CS0000001", Type: "case"}}
	h := opencases.SendReport(fakeSearcher{cases: found}, email, []string{"jane.doe@example.com"}, nil, "", true)
	if err := h(context.Background()); err != nil {
		t.Fatal(err)
	}
	if email.calls != 1 || email.body == "" {
		t.Errorf("calls = %d", email.calls)
	}
}

func TestSendReportSearchErrorIsReturned(t *testing.T) {
	email := &fakeSender{}
	h := opencases.SendReport(fakeSearcher{err: errors.New("boom")}, email, []string{"jane.doe@example.com"}, nil, "", true)
	if err := h(context.Background()); err == nil || email.calls != 0 {
		t.Errorf("err=%v calls=%d", err, email.calls)
	}
}
