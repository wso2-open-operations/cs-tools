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
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/caseevents"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/stream"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	// All upstream service clients authenticate as the same OAuth2
	// client-credentials app; only the base URL and scopes differ.
	oauth2ClientID := mustEnv("OAUTH2_CLIENT_ID")
	oauth2ClientSecret := mustEnv("OAUTH2_CLIENT_SECRET")
	oauth2TokenURL := mustEnv("OAUTH2_TOKEN_URL")

	// Entity client — used by the SSE handler for the upstream GetCase
	// ACL check before a caller subscribes to a case's activity stream.
	customerEntityCfg := entity.CustomerEntityConfig{
		BaseURL:      mustEnv("CUSTOMER_ENTITY_BASE_URL"),
		TokenURL:     oauth2TokenURL,
		ClientID:     oauth2ClientID,
		ClientSecret: oauth2ClientSecret,
		Scopes:       splitComma(os.Getenv("CUSTOMER_ENTITY_SCOPES")),
	}
	customerEntityClient := entity.NewCustomerEntityClient(customerEntityCfg)

	// Event Hub — optional. If unset, the service starts but only serves
	// the health check; the SSE endpoint returns 503 (no broadcast hub,
	// no consumer). When set, the service:
	//   - creates a BroadcastHub for in-process pub-sub
	//   - starts a Consumer in its own per-replica consumer group (LatestOffset)
	//   - runs the caseevents.Handler to fan case.comment_added / status_changed to
	//     the BroadcastHub
	var caseEventsConsumer *eventbus.Consumer
	var activityHub *stream.BroadcastHub

	if broker := os.Getenv("EVENT_HUB_BROKER"); broker != "" {
		eventBusCfg := eventbus.Config{
			Broker:           broker,
			ConnectionString: mustEnv("EVENT_HUB_CONNECTION_STRING"),
			Topic:            mustEnv("EVENT_HUB_TOPIC"),
		}
		// Every replica gets its own consumer group (a suffix on top of the
		// configured/default base name) rather than sharing one, so each
		// replica sees 100% of events instead of Kafka load-balancing them
		// across replicas — a shared group would mean an SSE client
		// connected to replica B never learns about an event a shared
		// group happened to hand to replica A. See internal/stream.
		// BroadcastHub: it only knows about subscribers on its own
		// process, so every replica must independently observe every event
		// to be able to broadcast it to whichever subscribers it happens to
		// be holding the connection for. Every ordinary rolling deploy (not
		// just a crash-restart) still means a brand new consumer group —
		// even newReplicaID's hostname-based suffix changes with every new
		// pod — so LatestOffset (not the package default) is required
		// here: EarliestOffset would replay the entire retained topic
		// history into a burst of stale case_updated notifications on
		// every deploy. This does mean an unbounded number of dead,
		// never-explicitly-deleted consumer groups accumulate on the
		// broker over the service's lifetime — an accepted tradeoff, since
		// per-replica uniqueness is required for the fan-out property
		// above and Event Hub's Kafka surface has no API this backend can
		// call to delete a consumer group it's done with.
		consumerGroupBase := envOrDefault("EVENT_HUB_CONSUMER_GROUP", "customer-portal-activity-stream-service")
		consumerGroup := fmt.Sprintf("%s-replica-%s", consumerGroupBase, newReplicaID())
		caseEventsConsumer = eventbus.NewConsumer(eventBusCfg, consumerGroup, eventbus.LatestOffset)
		activityHub = stream.NewBroadcastHub()
	}

	// SSE handler — depends on entity client + optional hub.
	streamHandler := handler.NewStreamHandler(customerEntityClient, activityHub)

	// Health check listener (:8080) — simple REST endpoint for Choreo liveness probe.
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	authCfg := middleware.Config{
		JWKSEndpoint:          mustEnv("AUTH_JWKS_ENDPOINT"),
		Issuer:                mustEnv("AUTH_ISSUER"),
		Audiences:             splitComma(mustEnv("AUTH_AUDIENCE")),
		ClockSkew:             5 * time.Second,
		TokenValidatorEnabled: os.Getenv("AUTH_TOKEN_VALIDATOR_ENABLED") != "false",
	}

	authMiddleware := middleware.Auth(authCfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Health check server
	healthAddr := ":" + mustPort("HEALTH_PORT", "8080")
	healthLn, err := (&net.ListenConfig{}).Listen(ctx, "tcp", healthAddr)
	if err != nil {
		slog.Error("failed to bind health listener", "addr", healthAddr, "err", err)
		os.Exit(1)
	}

	healthSrv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CORS(splitComma(os.Getenv("CORS_ALLOWED_ORIGINS")))(
				middleware.CorrelationID(
					authMiddleware(
						middleware.Logger(healthMux),
					),
				),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := healthSrv.Serve(healthLn); err != nil && err != http.ErrServerClosed {
			slog.Error("health server exited", "err", err)
			os.Exit(1)
		}
	}()
	slog.Info("health server started", "addr", healthLn.Addr().String())

	// Stream listener (:9092) always binds and serves the route, regardless
	// of whether Event Hub is configured — StreamCaseActivities itself
	// returns 503 when activityHub is nil, which is the documented
	// degraded-mode response. Binding only conditionally would instead
	// fail the connection before the handler ever runs, since Choreo's
	// component config always exposes this port. Only the consumer/hub
	// machinery above is conditional. The SSE connection must stay open
	// indefinitely, so WriteTimeout/IdleTimeout are disabled; mirror of the
	// sibling csm-portal-activity-stream-service's own stream listener.
	streamMux := http.NewServeMux()
	streamMux.HandleFunc("GET /cases/{id}/activities/stream", streamHandler.StreamCaseActivities)

	streamAddr := ":" + mustPort("STREAM_PORT", "9092")
	var streamLn net.Listener
	streamLn, err = (&net.ListenConfig{}).Listen(ctx, "tcp", streamAddr)
	if err != nil {
		slog.Error("failed to bind stream listener", "addr", streamAddr, "err", err)
		os.Exit(1)
	}

	streamSrv := &http.Server{
		// SecurityHeaders must stay outermost so its headers are present
		// on every response, including a CORS preflight — CORS runs
		// next, still ahead of Auth: a browser preflight carries no
		// x-jwt-assertion header, so Auth must never see it first. See
		// middleware.CORS's doc comment.
		// STREAM_CORS_ALLOWED_ORIGINS is a comma-separated allow-list;
		// unset denies all cross-origin requests (fail-closed — see
		// middleware.CORS's doc comment for why).
		Handler: middleware.SecurityHeaders(
			middleware.CORS(splitComma(os.Getenv("STREAM_CORS_ALLOWED_ORIGINS")))(
				middleware.CorrelationID(
					authMiddleware(
						middleware.Logger(streamMux),
					),
				),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       0,
	}

	go func() {
		if err := streamSrv.Serve(streamLn); err != nil && err != http.ErrServerClosed {
			slog.Error("stream server exited", "err", err)
			os.Exit(1)
		}
	}()
	slog.Info("case-activity stream server started", "addr", streamLn.Addr().String())

	if caseEventsConsumer != nil {
		go caseEventsConsumer.Run(ctx, caseevents.NewHandler(activityHub).Handle)
		slog.Info("case-events consumer started")
	}

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Both listeners get the shutdown goroutine's own use of shutdownCtx,
	// running concurrently rather than one after the other.
	var wg sync.WaitGroup
	wg.Go(func() {
		if err := streamSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("stream server graceful shutdown failed", "err", err)
		}
	})
	var healthSrvErr error
	wg.Go(func() {
		healthSrvErr = healthSrv.Shutdown(shutdownCtx)
	})
	wg.Wait()
	if healthSrvErr != nil {
		slog.Error("graceful shutdown failed", "err", healthSrvErr)
		os.Exit(1)
	}

	if caseEventsConsumer != nil {
		caseEventsConsumer.Close()
	}

	slog.Info("Customer Portal Activity Stream Service stopped")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// newReplicaID returns a stable identifier for this process's Kafka
// consumer group suffix (see the EVENT_HUB_BROKER block above), preferring
// the container/pod hostname — in Kubernetes/Choreo this is the pod name by
// default, requiring no extra deployment config — over a fresh random ID.
// A random ID on every plain process restart would make Event Hub treat it
// as a brand new, never-before-seen consumer group every time: NewConsumer
// starts new groups at kafka.FirstOffset, so the restart would replay every
// retained event and re-broadcast stale case_updated notifications to
// whichever SSE clients happen to be connected. The hostname is only stable
// within the same pod's lifetime, so an actual redeploy (new pod, new
// hostname) still causes a one-time replay — an accepted, much rarer
// tradeoff than replaying on every restart.
//
// Falls back to a random UUID v4 (deliberately duplicating
// middleware.newCorrelationID's same crypto/rand-based approach rather than
// exporting/sharing it — the two ids serve unrelated purposes and don't
// need to be coupled by a shared helper) only if the hostname is
// unavailable, which is rare outside unusual sandboxed environments.
func newReplicaID() string {
	if host, err := os.Hostname(); err == nil && host != "" {
		return host
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		slog.Error("failed to generate replica id", "err", err)
		os.Exit(1)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// mustPort returns the value of the given environment variable (or def if
// unset) as a bare port number, e.g. "8080" — not an address like ":8080" or
// "localhost:8080". Exits the process if the value isn't a valid TCP port.
func mustPort(key, def string) string {
	v := envOrDefault(key, def)
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		slog.Error("environment variable must be a plain port number (e.g. \"8080\"), not an address", "key", key, "value", v)
		os.Exit(1)
	}
	return v
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Silently ignored if the file does not exist; logs a warning for any other error.
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
		// Strip surrounding quotes from value.
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
