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
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/paging"
)

// caseAlertSettingsSource is PAGING_CONFIG_SOURCE: whether the chat rooms,
// group routes and ladder values saved on the Team Schedule's Alert Settings tab
// are used.
//
//	db (or unset)   they replace what they name, for every ladder -- CRE, SRE
//	                and SME; everything else is the file's. Nothing saved is
//	                the file as it is.
//	off             the file and env vars only, exactly as before. entity-service
//	                is not asked.
//	shadow          they are read and compared with the file's, and every
//	                difference is logged, but the file's are used.
//
// An env var rather than a file key on purpose: the file is decoded strictly,
// and a new key would turn paging off on any build that does not know it.
const (
	caseAlertSourceOff    = "off"
	caseAlertSourceShadow = "shadow"
	caseAlertSourceDB     = "db"
)

// caseAlertSettingsReader is the entity-service calls used here.
type caseAlertSettingsReader interface {
	CaseAlertSettings(ctx context.Context) (paging.CaseAlertSettings, error)
	ReportCaseAlertSettingsApplied(ctx context.Context, revision int64, mode string, summary map[string]any) error
}

// caseAlertSnapshots keeps the last settings a db start loaded, so a start
// while entity-service cannot be read still runs with them. Load returns nil
// when there are none.
type caseAlertSnapshots interface {
	Load(ctx context.Context) ([]byte, error)
	Save(ctx context.Context, b []byte) error
}

// caseAlertStart is how hard a db start tries before it gives up on
// entity-service: how long it keeps retrying (PAGING_CONFIG_WAIT), and where
// the last good settings are kept. The zero value tries once and keeps none.
type caseAlertStart struct {
	wait      time.Duration
	snapshots caseAlertSnapshots
	// sleep waits between tries; nil is a real wait.
	sleep func(context.Context, time.Duration) error
}

const (
	defaultCaseAlertWait = 2 * time.Minute
	caseAlertReadTimeout = 15 * time.Second
)

// caseAlertSettingsMode reads PAGING_CONFIG_SOURCE: unset is db. Anything it
// does not know is off, logged, so a typo can never change what is paged.
func caseAlertSettingsMode(raw string) string {
	switch m := strings.ToLower(strings.TrimSpace(raw)); m {
	case "":
		return caseAlertSourceDB
	case caseAlertSourceOff, caseAlertSourceShadow, caseAlertSourceDB:
		return m
	default:
		slog.Error("PAGING_CONFIG_SOURCE is not db, off or shadow; the file and env vars are used", "value", raw)
		return caseAlertSourceOff
	}
}

// caseAlertWait reads PAGING_CONFIG_WAIT: how long a db start keeps trying
// entity-service. Unset is two minutes; 0 tries once.
func caseAlertWait(raw string) time.Duration {
	v := strings.TrimSpace(raw)
	if v == "" {
		return defaultCaseAlertWait
	}
	d, err := time.ParseDuration(v)
	if v == "0" {
		d, err = 0, nil
	}
	if err != nil || d < 0 {
		slog.Warn("PAGING_CONFIG_WAIT is not a duration; using the default", "value", raw, "default", defaultCaseAlertWait)
		return defaultCaseAlertWait
	}
	return d
}

// applyCaseAlertSettings returns the escalation configuration to run with.
// Off returns cfg untouched without asking anyone. Otherwise the saved
// settings are read once (db: retried for start.wait); when they cannot be
// read, a db start uses the last ones it loaded, without their URLs, and
// otherwise cfg. When they would not pass the file's own checks, cfg is used
// as it is -- never the defaults, which turn paging off. setenv puts a saved
// room's URL where the engine reads webhooks from (db mode only).
func applyCaseAlertSettings(ctx context.Context, mode string, cfg paging.Config, entity caseAlertSettingsReader, setenv func(string, string) error, start caseAlertStart) paging.Config {
	cfg, _ = loadCaseAlertSettings(ctx, mode, cfg, entity, setenv, start)
	return cfg
}

// caseAlertLoaded is which saved revision a start read from entity-service.
// watch is true only for a db start that really read it: a start from the
// last-loaded snapshot or the file never restarts itself (see
// caseAlertWatch), so an entity-service that keeps failing cannot turn into
// a restart loop.
type caseAlertLoaded struct {
	revision int64
	watch    bool
}

// loadCaseAlertSettings is applyCaseAlertSettings, also saying which saved
// revision this process runs with.
func loadCaseAlertSettings(ctx context.Context, mode string, cfg paging.Config, entity caseAlertSettingsReader, setenv func(string, string) error, start caseAlertStart) (paging.Config, caseAlertLoaded) {
	if mode == caseAlertSourceOff {
		return cfg, caseAlertLoaded{}
	}
	if entity == nil {
		slog.Warn("PAGING_CONFIG_SOURCE needs CUSTOMER_ENTITY_BASE_URL; the file and env vars are used", "mode", mode)
		return cfg, caseAlertLoaded{}
	}
	wait := time.Duration(0)
	if mode == caseAlertSourceDB {
		wait = start.wait
	}
	saved, err := readCaseAlertSettings(ctx, entity, wait, start.sleep)
	fromSnapshot := false
	if err != nil {
		if mode == caseAlertSourceDB && start.snapshots != nil {
			if snap, at, ok := loadCaseAlertSnapshot(ctx, start.snapshots); ok {
				saved, fromSnapshot = snap, true
				slog.Warn("could not read the Case Alerts settings; using the last ones this service loaded, with the file's chat rooms",
					"err", err, "revision", snap.Revision, "loadedAt", at)
			}
		}
		if !fromSnapshot {
			slog.Warn("could not read the Case Alerts settings; the file and env vars are used", "mode", mode, "err", err)
			return cfg, caseAlertLoaded{}
		}
	}
	// Watched whatever the merge makes of them: a save that fixes a refused
	// value moves the revision too, and should be picked up.
	loaded := caseAlertLoaded{revision: saved.Revision, watch: mode == caseAlertSourceDB && !fromSnapshot}

	merged, env, changes, mergeErr := paging.MergeCaseAlertSettings(cfg, saved)
	summary := map[string]any{"rooms": len(saved.Rooms), "routes": len(saved.Routes), "values": len(saved.Values), "changes": changes}
	report := func(applied string, running paging.Config, savedKeys map[string]map[string]bool) {
		if fromSnapshot {
			// entity-service could not be read; there is nobody to tell.
			return
		}
		if applied != caseAlertSourceOff {
			summary["running"] = paging.RunningCaseAlertValues(running, savedKeys)
		}
		reportCtx, cancel := context.WithTimeout(ctx, caseAlertReadTimeout)
		defer cancel()
		if err := entity.ReportCaseAlertSettingsApplied(reportCtx, saved.Revision, applied, summary); err != nil {
			slog.Warn("could not report the Case Alerts settings this process loaded", "err", err)
		}
	}
	if mergeErr != nil {
		summary["error"] = mergeErr.Error()
		slog.Error("the saved Case Alerts settings would not pass the file's checks; the file and env vars are used",
			"mode", mode, "revision", saved.Revision, "err", mergeErr)
		report(caseAlertSourceOff, cfg, nil)
		return cfg, loaded
	}

	if mode == caseAlertSourceShadow {
		for _, c := range changes {
			slog.Info("case alerts shadow: the saved settings would change", "change", c, "revision", saved.Revision)
		}
		slog.Info("case alerts shadow: compared the saved settings with the file; the file's are used",
			"revision", saved.Revision, "differences", len(changes))
		report(caseAlertSourceShadow, cfg, nil)
		return cfg, loaded
	}

	names := make([]string, 0, len(env))
	for name, url := range env {
		if err := setenv(name, url); err != nil {
			slog.Error("could not put a saved Case Alerts room in the environment; the file and env vars are used", "variable", name)
			report(caseAlertSourceOff, cfg, nil)
			return cfg, loaded
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, c := range changes {
		slog.Info("case alerts: using a saved setting", "change", c, "revision", saved.Revision)
	}
	slog.Info("case alerts: using the saved settings over the file's", "revision", saved.Revision,
		"rooms", names, "values", len(saved.Values), "changes", len(changes), "fromLastLoaded", fromSnapshot)
	report(caseAlertSourceDB, merged, paging.SavedCaseAlertKeys(saved))
	if !fromSnapshot && start.snapshots != nil {
		saveCaseAlertSnapshot(ctx, start.snapshots, saved)
	}
	return merged, loaded
}

// readCaseAlertSettings reads the saved settings, trying again for up to wait
// when entity-service is unreachable, failing (5xx) or busy (429). A refusal
// (another 4xx) is not tried again: waiting does not change it.
func readCaseAlertSettings(ctx context.Context, entity caseAlertSettingsReader, wait time.Duration, sleep func(context.Context, time.Duration) error) (paging.CaseAlertSettings, error) {
	if sleep == nil {
		sleep = sleepCtx
	}
	deadline := time.Now().Add(wait)
	pause := 2 * time.Second
	for try := 1; ; try++ {
		readCtx, cancel := context.WithTimeout(ctx, caseAlertReadTimeout)
		saved, err := entity.CaseAlertSettings(readCtx)
		cancel()
		if err == nil {
			return saved, nil
		}
		left := time.Until(deadline)
		if !retryableCaseAlertRead(err) || left <= 0 {
			return paging.CaseAlertSettings{}, err
		}
		// The last wait is cut to the deadline, so there is always one try
		// at the end of PAGING_CONFIG_WAIT rather than giving up a pause early.
		wait := min(pause, left)
		slog.Warn("could not read the Case Alerts settings; trying again", "try", try, "in", wait, "err", err)
		if serr := sleep(ctx, wait); serr != nil {
			return paging.CaseAlertSettings{}, err
		}
		pause = min(pause*2, 30*time.Second)
	}
}

func retryableCaseAlertRead(err error) bool {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= http.StatusInternalServerError || apiErr.StatusCode == http.StatusTooManyRequests
	}
	return true
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// caseAlertSnapshot is the last settings a db start loaded, as kept. The
// rooms' webhook URLs are never in it: they stay in entity-service, so a
// start from it posts to the file's rooms.
type caseAlertSnapshot struct {
	LoadedAt time.Time                `json:"loadedAt"`
	Settings paging.CaseAlertSettings `json:"settings"`
}

func saveCaseAlertSnapshot(ctx context.Context, store caseAlertSnapshots, s paging.CaseAlertSettings) {
	kept := s
	kept.Rooms = make([]paging.CaseAlertRoom, len(s.Rooms))
	for i, r := range s.Rooms {
		r.WebhookURL = ""
		kept.Rooms[i] = r
	}
	b, err := json.Marshal(caseAlertSnapshot{LoadedAt: time.Now().UTC(), Settings: kept})
	if err == nil {
		saveCtx, cancel := context.WithTimeout(ctx, caseAlertReadTimeout)
		defer cancel()
		err = store.Save(saveCtx, b)
	}
	if err != nil {
		slog.Warn("could not keep the Case Alerts settings for a start without entity-service", "err", err)
	}
}

func loadCaseAlertSnapshot(ctx context.Context, store caseAlertSnapshots) (paging.CaseAlertSettings, time.Time, bool) {
	loadCtx, cancel := context.WithTimeout(ctx, caseAlertReadTimeout)
	defer cancel()
	b, err := store.Load(loadCtx)
	if err != nil {
		slog.Warn("could not read the last Case Alerts settings this service loaded", "err", err)
		return paging.CaseAlertSettings{}, time.Time{}, false
	}
	if b == nil {
		return paging.CaseAlertSettings{}, time.Time{}, false
	}
	var snap caseAlertSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		slog.Warn("the last Case Alerts settings this service loaded could not be read", "err", err)
		return paging.CaseAlertSettings{}, time.Time{}, false
	}
	return snap.Settings, snap.LoadedAt, true
}

// redisCaseAlertSnapshots keeps the snapshot in the Redis the ladders already
// keep their state in.
type redisCaseAlertSnapshots struct{ rdb *redis.Client }

const caseAlertSnapshotKey = "incident:escalation:case-alerts:last-loaded"

func (r redisCaseAlertSnapshots) Load(ctx context.Context) ([]byte, error) {
	b, err := r.rdb.Get(ctx, caseAlertSnapshotKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return b, err
}

func (r redisCaseAlertSnapshots) Save(ctx context.Context, b []byte) error {
	return r.rdb.Set(ctx, caseAlertSnapshotKey, b, 0).Err()
}

// caseAlertStartFor is how a db start reads the settings in this deployment:
// PAGING_CONFIG_WAIT, and the snapshot in Redis when there is one.
func caseAlertStartFor(rdb *redis.Client) caseAlertStart {
	start := caseAlertStart{wait: caseAlertWait(os.Getenv("PAGING_CONFIG_WAIT"))}
	if rdb != nil {
		start.snapshots = redisCaseAlertSnapshots{rdb: rdb}
	}
	return start
}

// caseAlertSettingsEntity is the entity-service client applyCaseAlertSettings
// reads with, or nil without CUSTOMER_ENTITY_BASE_URL.
func caseAlertSettingsEntity() caseAlertSettingsReader {
	base := os.Getenv("CUSTOMER_ENTITY_BASE_URL")
	if base == "" {
		return nil
	}
	return paging.NewEntityClient(paging.EntityConfig{
		BaseURL:      base,
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("CUSTOMER_ENTITY_SCOPES")),
	})
}
