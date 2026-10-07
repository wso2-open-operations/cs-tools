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
	"sync"
	"time"
)

// pinger is the part of *pgxpool.Pool a Probe needs.
type pinger interface {
	Ping(ctx context.Context) error
}

// Probe answers database health checks, reusing one ping result for every interval so frequent probes hold at most one connection.
type Probe struct {
	pool    pinger
	timeout time.Duration
	every   time.Duration

	mu  sync.Mutex
	at  time.Time
	err error
}

// NewProbe returns a Probe that pings pool with timeout, at most once per every.
func NewProbe(pool pinger, timeout, every time.Duration) *Probe {
	return &Probe{pool: pool, timeout: timeout, every: every}
}

// Check returns nil when Postgres answered a ping within the last interval.
func (p *Probe) Check(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.at.IsZero() && time.Since(p.at) < p.every {
		return p.err
	}
	// Detached from the caller so a probe that hangs up early can't cache a cancellation as an outage.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.timeout)
	defer cancel()
	p.err = p.pool.Ping(ctx)
	p.at = time.Now()
	return p.err
}
