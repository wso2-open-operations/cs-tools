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

// Command acp-closure runs one full ACP evaluation pass and exits — the
// Choreo Scheduled/Manual Task's cron owns the schedule, not this process.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/emailservice"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/notify"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/sweep"
)

// projectUpdater is declared locally so main can hold either the real
// *entity.Client or *sweep.DryRunProjectUpdater in one variable — Go
// interface satisfaction is structural, so neither concrete type needs to
// reference this declaration.
type projectUpdater interface {
	UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error)
}

// notifier is declared locally, mirroring projectUpdater above, so main can
// hold either *notify.LoggingNotifier or *notify.EmailNotifier in one
// variable depending on SEND_REAL_EMAILS.
type notifier interface {
	Send(ctx context.Context, n notify.Notice) (delivered bool, err error)
}

func main() {
	loadDotEnv(".env")

	dryRun := envBool("DRY_RUN", true)
	sendRealEmails := envBool("SEND_REAL_EMAILS", false)
	testProjectID := os.Getenv("TEST_PROJECT_ID")
	excludedProjectIDs := parseExcludedProjectIDs(os.Getenv("EXCLUDED_PROJECT_IDS"))
	runID := newRunID()

	slog.Info("acp-closure-service starting",
		"runID", runID,
		"dryRun", dryRun,
		"sendRealEmails", sendRealEmails,
		"testProjectID", testProjectID,
		"excludedProjectIDs", sortedKeys(excludedProjectIDs),
	)

	entityClient := entity.NewClient(entity.Config{
		BaseURL:      mustEnv("CSM_INTEGRATION_BASE_URL"),
		TokenURL:     mustEnv("CSM_INTEGRATION_TOKEN_URL"),
		ClientID:     mustEnv("CSM_INTEGRATION_CLIENT_ID"),
		ClientSecret: mustEnv("CSM_INTEGRATION_CLIENT_SECRET"),
		Scopes:       strings.Fields(mustEnv("CSM_INTEGRATION_SCOPES")),
	})

	var updater projectUpdater = entityClient
	if dryRun {
		updater = &sweep.DryRunProjectUpdater{}
	}

	var ntf notifier = &notify.LoggingNotifier{Logger: slog.Default()}
	if sendRealEmails {
		emailClient, err := emailservice.NewClient(emailservice.Config{
			BaseURL:      mustEnv("EMAIL_SERVICE_BASE_URL"),
			TokenURL:     mustEnv("EMAIL_SERVICE_TOKEN_URL"),
			ClientID:     mustEnv("EMAIL_SERVICE_CLIENT_ID"),
			ClientSecret: mustEnv("EMAIL_SERVICE_CLIENT_SECRET"),
			FromAddress:  mustEnv("EMAIL_SERVICE_FROM_ADDRESS"),
		})
		if err != nil {
			slog.Error("invalid email service configuration", "err", err)
			os.Exit(1)
		}
		ntf = &notify.EmailNotifier{
			Sender:                 emailClient,
			Logger:                 slog.Default(),
			AllowNonWSO2Recipients: envBool("EMAIL_SERVICE_ALLOW_NON_WSO2_RECIPIENTS", false),
		}
	}

	ctx := entity.WithCorrelationID(context.Background(), runID)

	result, err := sweep.Run(ctx, entityClient, updater, ntf, time.Now(), testProjectID, excludedProjectIDs)
	if err != nil {
		slog.Error("acp-closure-service sweep failed", "runID", runID, "err", err)
		os.Exit(1)
	}

	slog.Info("acp-closure-service finished",
		"runID", runID,
		"dryRun", dryRun,
		"projectsEvaluated", result.ProjectsEvaluated,
		"projectsExcluded", result.ProjectsExcluded,
		"failureCount", len(result.Failures),
	)
	for _, f := range result.Failures {
		slog.Error("project failed", "runID", runID, "projectID", f.ProjectID, "err", f.Err)
	}

	os.Exit(exitCode(len(result.Failures)))
}

// exitCode reports the process exit status for a completed sweep. A
// scheduled Choreo task relies on the exit code as its alerting signal, so
// any project failure — not just a fatal sweep-level error — must be
// reported as non-zero; a fully green run is the only case that exits 0.
func exitCode(failureCount int) int {
	if failureCount > 0 {
		return 1
	}
	return 0
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

// envBool parses key as a bool, defaulting to def on anything except a
// successfully-parsed value — unset, empty, or malformed all fall back to
// def rather than erroring. Used for DRY_RUN specifically so any
// misconfiguration fails toward dry-run, not toward real writes.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("invalid boolean environment variable, using default", "key", key, "value", v, "default", def)
		return def
	}
	return parsed
}

// parseExcludedProjectIDs parses EXCLUDED_PROJECT_IDS: a comma-separated
// list of project IDs the sweep should skip entirely, never fetching or
// evaluating them (backs sweep.Run's excludedProjectIDs parameter). Per
// explicit design direction (PR #1440 discussion, Sajith Ekanayake): this
// is meant for deliberate, verified business exclusions — expected to be
// empty in production almost all the time, populated more often in
// dev/staging to keep a known-broken project out of the way. Not a
// substitute for fixing genuine data issues; a project put here produces
// zero signal about whatever might actually be wrong with it. Each entry
// is trimmed of surrounding whitespace; empty entries (from a stray comma,
// or an entirely empty/unset value) are dropped rather than matching "".
func parseExcludedProjectIDs(v string) map[string]bool {
	ids := map[string]bool{}
	for raw := range strings.SplitSeq(v, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		ids[id] = true
	}
	return ids
}

// sortedKeys returns m's keys in sorted order, for stable, readable log
// output — map iteration order is randomized in Go, which would make the
// same EXCLUDED_PROJECT_IDS configuration log differently across runs.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// loadDotEnv reads a .env file and sets any unset environment variables from
// it. Silently ignored if the file does not exist; logs a warning for any
// other error.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, set := os.LookupEnv(k); !set {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}

// newRunID generates a UUID v4 identifying this invocation, forwarded as the
// correlation ID on every outbound call so a full run's log lines and
// upstream requests can be tied together.
func newRunID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("main: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
