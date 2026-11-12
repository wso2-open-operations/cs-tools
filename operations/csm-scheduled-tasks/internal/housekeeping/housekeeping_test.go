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

package housekeeping

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
)

type fakeCleaner struct {
	result ledger.AttachmentCleanup
	err    error
	calls  int
}

func (f *fakeCleaner) CleanupAttachmentUploads(context.Context) (ledger.AttachmentCleanup, error) {
	f.calls++
	return f.result, f.err
}

func TestCleanupAttachmentUploads_RunsTheCleanupOnce(t *testing.T) {
	client := &fakeCleaner{result: ledger.AttachmentCleanup{Cleaned: 2, ImagesRemoved: 1}}
	if err := CleanupAttachmentUploads(client)(context.Background()); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, want 1", client.calls)
	}
}

// Files that could not be removed now are tried again next run, so they do not fail the run.
func TestCleanupAttachmentUploads_PartialFailureDoesNotFailTheRun(t *testing.T) {
	client := &fakeCleaner{result: ledger.AttachmentCleanup{Failed: 1, ImagesFailed: 2}}
	if err := CleanupAttachmentUploads(client)(context.Background()); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestCleanupAttachmentUploads_ACallFailureFailsTheRun(t *testing.T) {
	client := &fakeCleaner{err: errors.New("entity-service is down")}
	if err := CleanupAttachmentUploads(client)(context.Background()); err == nil {
		t.Error("err = nil, want the run to fail so it is retried")
	}
}
