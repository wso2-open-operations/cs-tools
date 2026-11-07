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
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/paging"
)

// Only a db start that really read the saved settings is watched: a start on
// the snapshot or the file must never restart itself, or an entity-service
// that keeps failing would become a restart loop.
func TestLoadCaseAlertSettings_WatchesOnlyADBStartThatReadTheSettings(t *testing.T) {
	refused := paging.CaseAlertSettings{Revision: 4, Routes: []paging.CaseAlertRoute{{Ladder: "SRE", GroupName: "apollo", RotaCode: "SRE_SAAS"}}}
	withABT := fileConfig()
	withABT.SRE.Teams.ABTs = []string{"apollo"}
	withABT.SRE.Teams.DefaultRota = "SRE_SAAS"
	kept := &memSnapshots{}
	applyCaseAlertSettings(context.Background(), caseAlertSourceDB, fileConfig(), &fakeCaseAlertReader{saved: savedApollo()}, envRecorder{}.set, caseAlertStart{snapshots: kept})
	down := func() *fakeCaseAlertReader { return &fakeCaseAlertReader{err: errors.New("connection refused")} }
	failSet := func(string, string) error { return errors.New("no") }

	cases := map[string]struct {
		mode   string
		cfg    paging.Config
		reader caseAlertSettingsReader
		setenv func(string, string) error
		start  caseAlertStart
		want   caseAlertLoaded
	}{
		"db, read":                  {caseAlertSourceDB, fileConfig(), &fakeCaseAlertReader{saved: savedApollo()}, envRecorder{}.set, caseAlertStart{}, caseAlertLoaded{revision: 9, watch: true}},
		"db, refused by the checks": {caseAlertSourceDB, withABT, &fakeCaseAlertReader{saved: refused}, envRecorder{}.set, caseAlertStart{}, caseAlertLoaded{revision: 4, watch: true}},
		"db, a room not put in env": {caseAlertSourceDB, fileConfig(), &fakeCaseAlertReader{saved: savedApollo()}, failSet, caseAlertStart{}, caseAlertLoaded{revision: 9, watch: true}},
		"db, from the last loaded":  {caseAlertSourceDB, fileConfig(), down(), envRecorder{}.set, caseAlertStart{snapshots: kept}, caseAlertLoaded{revision: 9}},
		"db, entity-service down":   {caseAlertSourceDB, fileConfig(), down(), envRecorder{}.set, caseAlertStart{}, caseAlertLoaded{}},
		"db, no entity-service":     {caseAlertSourceDB, fileConfig(), nil, envRecorder{}.set, caseAlertStart{}, caseAlertLoaded{}},
		"shadow uses the file":      {caseAlertSourceShadow, fileConfig(), &fakeCaseAlertReader{saved: savedApollo()}, envRecorder{}.set, caseAlertStart{}, caseAlertLoaded{revision: 9}},
		"off asks nobody":           {caseAlertSourceOff, fileConfig(), &fakeCaseAlertReader{saved: savedApollo()}, envRecorder{}.set, caseAlertStart{}, caseAlertLoaded{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, got := loadCaseAlertSettings(context.Background(), c.mode, c.cfg, c.reader, c.setenv, c.start)
			if got != c.want {
				t.Fatalf("loaded %+v, want %+v", got, c.want)
			}
		})
	}
}

// watchClock is a fake clock: every sleep moves it on, and each read takes
// the next revision; when they run out the context ends.
type watchClock struct {
	t      time.Time
	revs   []any // int64 or error
	reads  int
	sleeps []time.Duration
	cancel context.CancelFunc
}

func (c *watchClock) now() time.Time { return c.t }

func (c *watchClock) sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.sleeps = append(c.sleeps, d)
	c.t = c.t.Add(d)
	return nil
}

func (c *watchClock) read(context.Context) (int64, error) {
	if c.reads >= len(c.revs) {
		c.cancel()
		return 0, context.Canceled
	}
	r := c.revs[c.reads]
	c.reads++
	if err, ok := r.(error); ok {
		return 0, err
	}
	return r.(int64), nil
}

// testWatch watches revision 9 in a process started an hour before the
// clock, polling every minute and settling for two.
func testWatch(revs ...any) (caseAlertWatch, *watchClock, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &watchClock{t: time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC), revs: revs, cancel: cancel}
	w := caseAlertWatch{
		loaded: 9, read: c.read,
		every: time.Minute, settle: 2 * time.Minute, minUptime: 10 * time.Minute, jitter: time.Minute,
		started: c.t.Add(-time.Hour), now: c.now, sleep: c.sleep,
	}
	return w, c, ctx
}

func TestCaseAlertWatch_NothingSavedNeverRestarts(t *testing.T) {
	w, c, ctx := testWatch(int64(9), int64(9), int64(9), int64(9), int64(9))
	if _, ok := w.run(ctx); ok {
		t.Fatal("restarted with nothing newer saved")
	}
	if c.reads != 5 {
		t.Fatalf("read %d times", c.reads)
	}
}

// A newer revision restarts the process once it has stayed unchanged for
// the settle time, counted from the read that first saw it.
func TestCaseAlertWatch_RestartsOnceANewerRevisionSettles(t *testing.T) {
	w, c, ctx := testWatch(int64(9), int64(10), int64(10), int64(10), int64(10))
	rev, ok := w.run(ctx)
	if !ok || rev != 10 {
		t.Fatalf("got %d, %v; want 10, true", rev, ok)
	}
	// Seen on the 2nd read, unchanged for two minutes by the 4th.
	if c.reads != 4 {
		t.Fatalf("restarted after %d reads, want 4", c.reads)
	}
}

// A burst of saves restarts once, after the last of them has settled.
func TestCaseAlertWatch_ABurstOfSavesRestartsOnce(t *testing.T) {
	w, c, ctx := testWatch(int64(10), int64(11), int64(12), int64(12), int64(12), int64(12))
	rev, ok := w.run(ctx)
	if !ok || rev != 12 || c.reads != 5 {
		t.Fatalf("got %d, %v after %d reads; want 12, true after 5", rev, ok, c.reads)
	}
}

// A process that has just started waits out the minimum uptime first: at
// most one self-restart per that long, whatever is saved.
func TestCaseAlertWatch_WaitsForTheMinimumUptime(t *testing.T) {
	revs := make([]any, 0, 15)
	for range 15 {
		revs = append(revs, int64(10))
	}
	w, c, ctx := testWatch(revs...)
	w.started = c.t
	rev, ok := w.run(ctx)
	if !ok || rev != 10 {
		t.Fatalf("got %d, %v", rev, ok)
	}
	if up := c.t.Sub(w.started); up < 10*time.Minute || up >= 11*time.Minute {
		t.Fatalf("restarted after %v up, want 10m", up)
	}
}

// A revision that cannot be read changes nothing: no restart on an error,
// and the settle time already counted is kept.
func TestCaseAlertWatch_AReadThatFailsChangesNothing(t *testing.T) {
	down := errors.New("connection refused")
	w, c, ctx := testWatch(down, down, down)
	if _, ok := w.run(ctx); ok || c.reads != 3 {
		t.Fatalf("restarted on errors (%d reads)", c.reads)
	}
	w, c, ctx = testWatch(int64(10), down, int64(10))
	if rev, ok := w.run(ctx); !ok || rev != 10 || c.reads != 3 {
		t.Fatalf("got %d, %v after %d reads; want 10, true after 3", rev, ok, c.reads)
	}
}

// Back to the loaded revision (nothing newer any more) clears what was seen.
func TestCaseAlertWatch_BackToTheLoadedRevisionDoesNotRestart(t *testing.T) {
	w, _, ctx := testWatch(int64(10), int64(9), int64(9), int64(9))
	if _, ok := w.run(ctx); ok {
		t.Fatal("restarted for a revision no longer saved")
	}
}

// The restart is put off by the random jitter, so replicas do not all stop
// together.
func TestCaseAlertWatch_WaitsTheJitterBeforeRestarting(t *testing.T) {
	w, c, ctx := testWatch(int64(10), int64(10), int64(10))
	w.pick = func(max time.Duration) time.Duration {
		if max != time.Minute {
			t.Fatalf("jitter max %v", max)
		}
		return 37 * time.Second
	}
	if _, ok := w.run(ctx); !ok {
		t.Fatal("did not restart")
	}
	if last := c.sleeps[len(c.sleeps)-1]; last != 37*time.Second {
		t.Fatalf("last wait %v, want the jitter", last)
	}
}

// A context that ends stops the watch without a restart.
func TestCaseAlertWatch_StopsWithTheContext(t *testing.T) {
	w, _, ctx := testWatch(int64(10), int64(10), int64(10))
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, ok := w.run(ctx); ok {
		t.Fatal("restarted after the context ended")
	}
}

// watchCaseAlertSettings starts nothing for a start that is not watched.
func TestWatchCaseAlertSettings_NotWatchedStartsNothing(t *testing.T) {
	called := false
	reader := &fakeCaseAlertReader{saved: paging.CaseAlertSettings{Revision: 99}}
	watchCaseAlertSettings(context.Background(), caseAlertLoaded{revision: 9}, reader, time.Now(), func() { called = true })
	watchCaseAlertSettings(context.Background(), caseAlertLoaded{revision: 9, watch: true}, nil, time.Now(), func() { called = true })
	time.Sleep(10 * time.Millisecond)
	if called || reader.reads != 0 {
		t.Fatalf("restart called %v, reads %d", called, reader.reads)
	}
}

// newCaseAlertWatch reads the revision from the settings entity-service
// returns.
func TestNewCaseAlertWatch_ReadsTheSavedRevision(t *testing.T) {
	w := newCaseAlertWatch(9, &fakeCaseAlertReader{saved: paging.CaseAlertSettings{Revision: 12}}, time.Now())
	if rev, err := w.read(context.Background()); err != nil || rev != 12 {
		t.Fatalf("read %d, %v", rev, err)
	}
	if w.every != caseAlertWatchEvery || w.settle != caseAlertWatchSettle || w.minUptime != caseAlertWatchMinUptime || w.pick(0) != 0 {
		t.Fatal("not the defaults")
	}
}
