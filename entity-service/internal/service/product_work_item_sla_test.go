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
	"slices"
	"testing"
	"time"
)

// fakeCreateSLA records which work items the create flow registers clocks for.
type fakeCreateSLA struct {
	registered []string
	err        error
}

func (f *fakeCreateSLA) RegisterWorkItemClocks(_ context.Context, id string) error {
	f.registered = append(f.registered, id)
	return f.err
}
func (f *fakeCreateSLA) ApplyWorkItemStatusEffects(context.Context, string, time.Time, bool) error {
	return nil
}
func (f *fakeCreateSLA) ReviseWorkItemClocks(context.Context, string, time.Time) error { return nil }
func (f *fakeCreateSLA) RecalculateWorkItemClocks(context.Context, string) error       { return nil }

func TestProductWorkItem_RegistersTheSLAClockAtCreation(t *testing.T) {
	e := newPWEnv()
	sla := &fakeCreateSLA{}
	e.svc = WithProductWorkItemSLA(e.svc, sla)
	resp, created, err := e.create(t, pwReq())
	if err != nil || !created || resp.WorkItem == nil {
		t.Fatalf("created=%v err=%v resp=%+v", created, err, resp)
	}
	if !slices.Equal(sla.registered, []string{resp.WorkItem.ID}) {
		t.Fatalf("registered = %v, want [%s]", sla.registered, resp.WorkItem.ID)
	}
}

func TestProductWorkItem_NoSLAClockForAnUntrackedPatch(t *testing.T) {
	e := newPWEnv()
	sla := &fakeCreateSLA{}
	e.svc = WithProductWorkItemSLA(e.svc, sla)
	req := pwReq()
	req.IssueTypeLabel = "Type/Patch"
	if _, _, err := e.create(t, req); err != nil {
		t.Fatal(err)
	}
	if len(sla.registered) != 0 {
		t.Fatalf("registered = %v, want none for a patch", sla.registered)
	}
}

func TestProductWorkItem_SLAFailureDoesNotFailTheCreate(t *testing.T) {
	e := newPWEnv()
	sla := &fakeCreateSLA{err: errors.New("database down")}
	e.svc = WithProductWorkItemSLA(e.svc, sla)
	resp, created, err := e.create(t, pwReq())
	if err != nil || !created || resp.Message != "GitHub issue created." {
		t.Fatalf("created=%v err=%v resp=%+v", created, err, resp)
	}
	if len(sla.registered) != 1 {
		t.Fatalf("registered = %v, want one attempt", sla.registered)
	}
}
