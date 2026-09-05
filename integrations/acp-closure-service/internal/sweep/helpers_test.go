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

package sweep

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/notify"
)

type mockEntityReader struct {
	searchAccountContactsFn func(ctx context.Context, accountID string, body []byte) ([]byte, error)
	searchProjectContactsFn func(ctx context.Context, projectID string, body []byte) ([]byte, error)
	searchProjectsFn        func(ctx context.Context, body []byte) ([]byte, error)
	searchProjectsCalls     [][]byte
	getProjectFn            func(ctx context.Context, id string) ([]byte, error)
	getAccountFn            func(ctx context.Context, id string) ([]byte, error)
	getAccountCalls         []string
}

func (m *mockEntityReader) GetAccount(ctx context.Context, id string) ([]byte, error) {
	m.getAccountCalls = append(m.getAccountCalls, id)
	if m.getAccountFn != nil {
		return m.getAccountFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityReader) GetProject(ctx context.Context, id string) ([]byte, error) {
	if m.getProjectFn != nil {
		return m.getProjectFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityReader) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	m.searchProjectsCalls = append(m.searchProjectsCalls, body)
	if m.searchProjectsFn != nil {
		return m.searchProjectsFn(ctx, body)
	}
	return []byte(`{"projects":[],"total":0,"limit":50,"offset":0,"hasMore":false}`), nil
}

func (m *mockEntityReader) SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error) {
	if m.searchAccountContactsFn != nil {
		return m.searchAccountContactsFn(ctx, accountID, body)
	}
	return []byte(`{"contacts":[]}`), nil
}

func (m *mockEntityReader) SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error) {
	if m.searchProjectContactsFn != nil {
		return m.searchProjectContactsFn(ctx, projectID, body)
	}
	return []byte(`{"contacts":[]}`), nil
}

type updateCall struct {
	id   string
	body []byte
}

type mockProjectUpdater struct {
	updateProjectFn func(ctx context.Context, id string, body []byte) ([]byte, error)
	calls           []updateCall
}

func (m *mockProjectUpdater) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	m.calls = append(m.calls, updateCall{id: id, body: body})
	if m.updateProjectFn != nil {
		return m.updateProjectFn(ctx, id, body)
	}
	return []byte(`{}`), nil
}

type mockNotifier struct {
	// sendFn, when set, decides both the delivered result and the error
	// for every call. Default (unset): delivered=false, err=nil — matches
	// LoggingNotifier's real behavior, and is what most tests want since
	// they're not exercising delivery-tracking specifically.
	sendFn func(ctx context.Context, n notify.Notice) (bool, error)
	sent   []notify.Notice
}

func (m *mockNotifier) Send(ctx context.Context, n notify.Notice) (bool, error) {
	m.sent = append(m.sent, n)
	if m.sendFn != nil {
		return m.sendFn(ctx, n)
	}
	return false, nil
}
