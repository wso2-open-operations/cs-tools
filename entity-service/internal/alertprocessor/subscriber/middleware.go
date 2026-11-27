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

package subscriber

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
)

// Middleware wraps a Handler with behaviour around every event.
type Middleware func(Handler) Handler

// Wrap applies mw to h, the first listed outermost.
func Wrap(h Handler, mw ...Middleware) Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// Recover turns a panic into an error, so one bad event is retried instead of stopping the subscriber.
func Recover() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, e event.Event) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("alert %s: panic: %v", e.AlertID, r)
				}
			}()
			return next(ctx, e)
		}
	}
}

// Log logs every failed event with its alert id and how long it took; ids only, never the alert.
func Log() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, e event.Event) error {
			start := time.Now()
			err := next(ctx, e)
			if err != nil {
				slog.ErrorContext(ctx, "alert subscriber: event failed, will retry", "alertId", e.AlertID, "took", time.Since(start), "err", err)
			}
			return err
		}
	}
}
