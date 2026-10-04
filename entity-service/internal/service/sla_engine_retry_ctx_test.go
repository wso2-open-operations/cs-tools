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

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSLAEngineService_ReviseCaseClocks_NoRetryOnceRequestIsDone proves the
// retry never holds the request: once the request's context is done there is
// no second attempt, and a failing attempt pair returns without a pause.
func TestSLAEngineService_ReviseCaseClocks_NoRetryOnceRequestIsDone(t *testing.T) {
	sev := domain.CaseSeverityCatastrophic

	repo := newRecordingSLAEngineRepo()
	repo.cancelErr = errors.New("db unavailable")
	repo.failReviseClocksTimes = -1
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	NewSLAEngineService(repo, nil).ReviseCaseClocks(ctx, "case-12", &sev, "")
	if repo.reviseClocksCalls != 1 {
		t.Fatalf("ReviseClocks calls = %d with a done context, want 1", repo.reviseClocksCalls)
	}

	repo = newRecordingSLAEngineRepo()
	repo.cancelErr = errors.New("db unavailable")
	repo.failReviseClocksTimes = -1
	start := time.Now()
	NewSLAEngineService(repo, nil).ReviseCaseClocks(context.Background(), "case-13", &sev, "")
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("two failing attempts took %v; the retry must not pause the request", elapsed)
	}
}
