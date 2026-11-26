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

package schedule

import (
	"context"
	"time"
)

type periodKey struct{}

// WithPeriod returns ctx carrying period: the period the engine claimed for
// the run it is about to make. engine.attempt sets it before calling a
// task's handler, so a handler that depends on its period reads the one the
// ledger holds, not one re-derived from the clock when the handler happens
// to run (which, after slow earlier tasks in the same tick, can already be
// the next period).
func WithPeriod(ctx context.Context, period time.Time) context.Context {
	return context.WithValue(ctx, periodKey{}, period)
}

// PeriodFromContext returns the period WithPeriod stored, and whether there
// was one. A manual run-now or a tryout runs a handler outside the engine,
// so it has none.
func PeriodFromContext(ctx context.Context) (time.Time, bool) {
	p, ok := ctx.Value(periodKey{}).(time.Time)
	return p, ok
}
