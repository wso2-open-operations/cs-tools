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

package main

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"
)

// The saved Case Alerts settings apply only at start-up: the engines are
// built from them, a ladder that is off has no consumer at all, and the SME
// page is wired once. So instead of swapping settings in a running process,
// a db start that read the saved settings watches their revision and, when a
// newer one has been saved, restarts itself through the normal shutdown; the
// platform starts it again and the new start loads them. Ladders in flight
// live in Redis and carry on after the restart, and a record delivered again
// is harmless (create-if-absent), so a restart only delays what was due
// while it ran.
const (
	// caseAlertWatchEvery is how often the revision is read: one small GET.
	caseAlertWatchEvery = time.Minute
	// caseAlertWatchSettle is how long a newer revision must stay unchanged
	// before restarting, so a burst of edits restarts once, not per save.
	caseAlertWatchSettle = 2 * time.Minute
	// caseAlertWatchMinUptime is the least a process runs before restarting
	// itself: at most one self-restart per this long.
	caseAlertWatchMinUptime = 10 * time.Minute
	// caseAlertWatchJitter is the most a restart is put off at random, so
	// several replicas do not all stop at the same moment.
	caseAlertWatchJitter = time.Minute
)

// caseAlertWatch decides when this process should restart to pick up saved
// Case Alerts settings.
type caseAlertWatch struct {
	// loaded is the revision this process runs with.
	loaded int64
	// read returns the revision saved now.
	read func(context.Context) (int64, error)

	every, settle, minUptime, jitter time.Duration
	started                          time.Time

	now   func() time.Time
	sleep func(context.Context, time.Duration) error
	// pick returns a random wait in [0, max); nil never waits.
	pick func(max time.Duration) time.Duration
}

// newCaseAlertWatch watches the revision entity-service holds, against the
// one loaded at start.
func newCaseAlertWatch(loaded int64, entity caseAlertSettingsReader, started time.Time) caseAlertWatch {
	return caseAlertWatch{
		loaded: loaded,
		read: func(ctx context.Context) (int64, error) {
			readCtx, cancel := context.WithTimeout(ctx, caseAlertReadTimeout)
			defer cancel()
			s, err := entity.CaseAlertSettings(readCtx)
			return s.Revision, err
		},
		every:     caseAlertWatchEvery,
		settle:    caseAlertWatchSettle,
		minUptime: caseAlertWatchMinUptime,
		jitter:    caseAlertWatchJitter,
		started:   started,
		now:       time.Now,
		sleep:     sleepCtx,
		pick: func(max time.Duration) time.Duration {
			if max <= 0 {
				return 0
			}
			return rand.N(max)
		},
	}
}

// run blocks until a restart is due, returning the revision saved now and
// true, or until ctx ends, returning false. A revision that cannot be read
// changes nothing: the process keeps running on what it has.
func (w caseAlertWatch) run(ctx context.Context) (int64, bool) {
	seen, seenAt := w.loaded, time.Time{}
	failing := false
	for {
		if err := w.sleep(ctx, w.every); err != nil {
			return 0, false
		}
		rev, err := w.read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return 0, false
			}
			if !failing {
				slog.Warn("case alerts: could not check for newly saved settings; trying again", "err", err)
			}
			failing = true
			continue
		}
		failing = false
		now := w.now()
		switch {
		case rev == w.loaded:
			// Nothing saved since start (or back to it): nothing to do.
			seen, seenAt = w.loaded, time.Time{}
			continue
		case rev != seen || seenAt.IsZero():
			seen, seenAt = rev, now
			slog.Info("case alerts: newer settings saved; this service restarts to apply them once no change has been saved for a while",
				"loaded", w.loaded, "saved", rev, "settle", w.settle)
			continue
		}
		if now.Sub(seenAt) < w.settle || now.Sub(w.started) < w.minUptime {
			continue
		}
		if w.pick != nil {
			if d := w.pick(w.jitter); d > 0 {
				if err := w.sleep(ctx, d); err != nil {
					return 0, false
				}
			}
		}
		return rev, true
	}
}

// watchCaseAlertSettings starts the watch for a db start that read the saved
// settings, and calls restart when one is due. It does nothing otherwise.
func watchCaseAlertSettings(ctx context.Context, loaded caseAlertLoaded, entity caseAlertSettingsReader, started time.Time, restart func()) {
	if !loaded.watch || entity == nil {
		return
	}
	w := newCaseAlertWatch(loaded.revision, entity, started)
	go func() {
		if rev, ok := w.run(ctx); ok {
			slog.Warn("case alerts: restarting to apply the saved settings", "loaded", loaded.revision, "saved", rev)
			restart()
		}
	}()
}
