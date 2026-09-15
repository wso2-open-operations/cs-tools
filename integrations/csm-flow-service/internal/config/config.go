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

// Package config loads the flow engine's configuration from the environment.
// The event bus is mandatory (mustEnv — the consumer's whole reason to exist),
// while entity-service and the DLQ are optional at construction and only fail
// on first use, matching the repo's config-strictness convention
// (docs/architecture.md §17.3). Every value is a flat single-line string:
// Choreo's config UI cannot deploy nested collections, which is also why flow
// definitions live in Postgres, not config.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Defaults.
const (
	defaultPort          = "8080"
	defaultConsumerGroup = "csm-flow-service"
)

// Config is the fully-resolved configuration.
type Config struct {
	// Event bus (required).
	EventHubBroker           string
	EventHubConnectionString string
	EventHubTopic            string
	ConsumerGroup            string

	// Dead-letter topic (optional). When empty, a record that exhausts its
	// retries is logged and dropped rather than dead-lettered.
	DLQTopic string

	// entity-service client (optional at startup; required by any flow that
	// calls it). Credentials are the shared OAuth2 app.
	EntityBaseURL string
	EntityScopes  []string
	OAuthClientID string
	OAuthSecret   string
	OAuthTokenURL string

	// EmailDebugRecipients (EMAIL_DEBUG_RECIPIENTS) redirects every notification
	// a flow would request to this list instead of the real resolved audience —
	// approval groups, project contacts, watchers. It exists so a dev or staging
	// deployment can be exercised end to end without mail reaching real
	// customers or internal groups.
	//
	// Mirrors csm-notification-service's EMAIL_DEBUG_MODE/EMAIL_DEBUG_RECIPIENTS
	// pair, with one deliberate simplification: there is no separate mode flag.
	// A non-empty list IS the switch. The pair exists there partly to guard the
	// mode=true-but-list-empty misconfiguration, which cannot arise when the
	// list alone decides.
	//
	// Recipient resolution still runs either way — only the final list is
	// swapped — so this never masks a broken entity-service lookup. And a flow
	// that resolves NO real recipients still sends nothing: debug mode must not
	// turn a would-be-silent event into mail.
	EmailDebugRecipients []string

	// DatabaseURL is the CSM Postgres the flows read: the records that trigger
	// them and the audiences they notify. Required — see cmd/consumer.
	DatabaseURL string

	// OutboxInterval is how often to poll event_outbox when the last drain came
	// back empty. A backlog drains at full speed regardless — this only governs
	// the idle case, so it trades notification latency against query volume.
	OutboxInterval time.Duration

	// HTTP health/metrics server.
	Port string
}

// Load reads and validates configuration from the environment.
func Load() (Config, error) {
	LoadDotEnv(".env")

	var missing []string
	must := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := Config{
		EventHubBroker:           must("EVENT_HUB_BROKER"),
		EventHubConnectionString: must("EVENT_HUB_CONNECTION_STRING"),
		EventHubTopic:            must("EVENT_HUB_TOPIC"),
		ConsumerGroup:            getenv("EVENT_HUB_CONSUMER_GROUP", defaultConsumerGroup),
		DLQTopic:                 strings.TrimSpace(os.Getenv("EVENT_HUB_DLQ_TOPIC")),

		EntityBaseURL: strings.TrimSpace(os.Getenv("CUSTOMER_ENTITY_BASE_URL")),
		EntityScopes:  splitScopes(os.Getenv("CUSTOMER_ENTITY_SCOPES")),
		OAuthClientID: strings.TrimSpace(os.Getenv("OAUTH2_CLIENT_ID")),
		OAuthSecret:   strings.TrimSpace(os.Getenv("OAUTH2_CLIENT_SECRET")),
		OAuthTokenURL: strings.TrimSpace(os.Getenv("OAUTH2_TOKEN_URL")),

		DatabaseURL:    must("DATABASE_URL"),
		OutboxInterval: envDuration("OUTBOX_POLL_INTERVAL", 5*time.Second),

		EmailDebugRecipients: splitScopes(os.Getenv("EMAIL_DEBUG_RECIPIENTS")),

		Port: getenv("PORT", defaultPort),
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("config: missing required env vars: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

// HasEntity reports whether the entity-service client is configured. A flow
// that needs entity-service should check this (or simply let the call fail) —
// the client is safe to construct either way.
func (c Config) HasEntity() bool {
	return c.EntityBaseURL != "" && c.OAuthClientID != "" && c.OAuthTokenURL != ""
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// splitScopes parses a space- or comma-separated scope list into a slice.
func splitScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ' ' || r == ',' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// atoiOr is a small helper for optional numeric env vars (kept for the
// consumer-count knobs added with Phase 1 scaling).
func atoiOr(raw string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
		return n
	}
	return def
}

// ensure atoiOr is retained even before its first caller lands.
var _ = atoiOr

// envDuration reads a Go duration string (e.g. "5s", "500ms"), falling back to
// def when unset or unparseable — a typo should not stop the service starting,
// only lose the override.
func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// LoadDotEnv reads a .env file from the working directory and sets any
// environment variable it names that is not already set. A real environment
// variable always wins, so a deployment is never overridden by a file that
// happened to be left in the image.
//
// Silently ignored when the file does not exist -- that is the deployed case,
// where Choreo supplies the environment directly. Exported because the dev
// tools (cmd/dryrun, cmd/publish) read the environment without going through
// Load, and having one of them work from .env while another did not is a
// confusing way to lose ten minutes.
//
// Mirrors csm-notification-service's own loadDotEnv rather than pulling in a
// dependency for twenty lines.
func LoadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- callers pass the literal ".env"
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("config: failed to open .env file", "err", err)
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
		// Strip surrounding quotes, so a value containing spaces or a ";" can
		// be written either way round.
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, present := os.LookupEnv(k); !present {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("config: error reading .env file", "err", err)
	}
}
