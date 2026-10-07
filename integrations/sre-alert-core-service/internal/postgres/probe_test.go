// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePinger struct {
	calls int
	err   error
}

func (f *fakePinger) Ping(context.Context) error {
	f.calls++
	return f.err
}

func TestProbe_ReusesResultWithinInterval(t *testing.T) {
	db := &fakePinger{err: errors.New("connection refused")}
	p := NewProbe(db, time.Second, time.Hour)
	for range 3 {
		if err := p.Check(context.Background()); err == nil {
			t.Fatal("Check = nil, want the ping error")
		}
	}
	if db.calls != 1 {
		t.Errorf("pings = %d, want 1 within the interval", db.calls)
	}
}

func TestProbe_PingsAgainAfterInterval(t *testing.T) {
	db := &fakePinger{err: errors.New("connection refused")}
	p := NewProbe(db, time.Second, time.Millisecond)
	if err := p.Check(context.Background()); err == nil {
		t.Fatal("Check = nil, want the ping error")
	}
	db.err = nil
	time.Sleep(5 * time.Millisecond)
	if err := p.Check(context.Background()); err != nil {
		t.Errorf("Check = %v after recovery, want nil", err)
	}
	if db.calls != 2 {
		t.Errorf("pings = %d, want 2", db.calls)
	}
}

func TestProbe_IgnoresCallerCancellation(t *testing.T) {
	db := &fakePinger{}
	p := NewProbe(db, time.Second, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Check(ctx); err != nil {
		t.Errorf("Check = %v, want nil since the ping itself succeeded", err)
	}
}
