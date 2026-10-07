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

package eventbus

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// Default restart backoff for Supervisor: doubles from min to max, and resets
// to min after a consumer has run for at least max without exiting.
const (
	DefaultRestartBackoffMin = time.Second
	DefaultRestartBackoffMax = 30 * time.Second
)

// Supervisor keeps one Consumer running for the life of ctx. A consumer
// whose Run returns while ctx is still live (e.g. its reader reported
// io.EOF) is closed, logged, and replaced by a fresh one from newConsumer
// after an exponential backoff — the process does not stay up as a replica
// whose stream endpoint accepts clients but never delivers another event.
// Running reports whether a consumer is currently inside Run, which the
// health endpoint exposes.
type Supervisor struct {
	newConsumer func() *Consumer
	handle      Handle
	backoffMin  time.Duration
	backoffMax  time.Duration

	running  atomic.Bool
	restarts atomic.Int64
}

// NewSupervisor constructs a Supervisor that builds consumers with
// newConsumer and feeds every record to handle.
func NewSupervisor(newConsumer func() *Consumer, handle Handle) *Supervisor {
	return &Supervisor{
		newConsumer: newConsumer,
		handle:      handle,
		backoffMin:  DefaultRestartBackoffMin,
		backoffMax:  DefaultRestartBackoffMax,
	}
}

// Running reports whether a consumer is currently running. False before Run
// starts, during a restart backoff, and after Run returns.
func (s *Supervisor) Running() bool { return s.running.Load() }

// Restarts returns how many times a consumer has been replaced after an
// unexpected exit.
func (s *Supervisor) Restarts() int64 { return s.restarts.Load() }

// Run blocks until ctx is canceled, running and restarting consumers as
// described on Supervisor. Every consumer it creates is closed before Run
// moves on or returns.
func (s *Supervisor) Run(ctx context.Context) {
	backoff := s.backoffMin
	for {
		c := s.newConsumer()
		started := time.Now()
		s.running.Store(true)
		err := c.Run(ctx, s.handle)
		s.running.Store(false)
		c.Close()

		if ctx.Err() != nil {
			return
		}
		if time.Since(started) >= s.backoffMax {
			backoff = s.backoffMin
		}
		s.restarts.Add(1)
		slog.ErrorContext(ctx, "eventbus: consumer exited; restarting after backoff", "err", err, "backoff", backoff, "restarts", s.restarts.Load())

		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		backoff = min(backoff*2, s.backoffMax)
	}
}
