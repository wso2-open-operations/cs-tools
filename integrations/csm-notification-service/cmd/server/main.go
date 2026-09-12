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
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/dispatch"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/slaengine"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/timecardengine"
)

func main() {
	loadDotEnv(".env")
	middleware.ConfigureLogger()

	// Email is not yet configured for every deployment, so its config is read
	// with os.Getenv (never mustEnv) — a missing or invalid configuration only
	// surfaces as an error the first time a caller requests the email
	// channel.
	//
	// Shares the same OAuth2 client credentials app
	// (OAUTH2_CLIENT_ID/OAUTH2_CLIENT_SECRET/OAUTH2_TOKEN_URL) as
	// customerEntityClient below, rather than a dedicated EMAIL_TOKEN_URL/
	// EMAIL_CLIENT_ID/EMAIL_CLIENT_SECRET — the real email-service deployment
	// this points at authenticates every caller through the same shared
	// gateway app, scoped via EMAIL_SCOPES, not a separate per-consumer app.
	// Only BaseURL/Scopes/FromAddress are specific to this client.
	emailClient := notifications.NewEmailClient(notifications.EmailConfig{
		BaseURL:      os.Getenv("EMAIL_BASE_URL"),
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("EMAIL_SCOPES")),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})

	// Google Chat is likewise optional per deployment; a missing or malformed
	// value logs a warning and yields no spaces rather than failing startup.
	googleChatClient := notifications.NewGoogleChatClient(notifications.GoogleChatConfig{
		Spaces: parseGoogleChatSpaces(os.Getenv("GOOGLE_CHAT_SPACES")),
	})

	// Twilio (the call channel, used by incident.created) is likewise
	// optional per deployment; a missing config only surfaces as an error
	// the first time dispatch requests it.
	twilioClient := notifications.NewTwilioClient(notifications.TwilioConfig{
		AccountSID:          os.Getenv("TWILIO_ACCOUNT_SID"),
		AuthToken:           os.Getenv("TWILIO_AUTH_TOKEN"),
		FromNumber:          os.Getenv("TWILIO_FROM_NUMBER"),
		MessagingServiceSid: os.Getenv("TWILIO_MESSAGING_SERVICE_SID"),
		Voice:               os.Getenv("TWILIO_VOICE"),
		Language:            os.Getenv("TWILIO_LANGUAGE"),
		APIBaseURL:          os.Getenv("TWILIO_API_BASE_URL"),
	})

	// The customer entity service backs per-recipient portal-link resolution
	// (internal/recipientlinks) — optional per deployment like the channel
	// clients above (os.Getenv, not mustEnv), but unlike them, an unset
	// config doesn't just make one channel unavailable: every case.* event's
	// SendEmail is behind ResolveLinks, so a missing config makes every
	// case.* email fail instead. Warn loudly at startup so a misconfigured
	// deployment doesn't discover this silently on its first real event.
	//
	// Shares the same OAuth2 client credentials app as emailClient above
	// (OAUTH2_CLIENT_ID/OAUTH2_CLIENT_SECRET/OAUTH2_TOKEN_URL) rather than
	// getting its own — mirroring apps/csm-portal/backend's own entity
	// client, which authenticates against the same entity-service this way.
	// Only BaseURL/Scopes are specific to this client.
	customerEntityClient := entity.NewCustomerEntityClient(entity.CustomerEntityConfig{
		BaseURL:      os.Getenv("CUSTOMER_ENTITY_BASE_URL"),
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("CUSTOMER_ENTITY_SCOPES")),
	})
	if os.Getenv("CUSTOMER_ENTITY_BASE_URL") == "" {
		slog.Warn("CUSTOMER_ENTITY_BASE_URL is not set; case.* emails will fail until it is configured")
	}
	customerPortalBaseURL := os.Getenv("CUSTOMER_PORTAL_WEB_BASE_URL")
	csmPortalBaseURL := os.Getenv("CSM_PORTAL_WEB_BASE_URL")
	// An empty base URL here doesn't fail link resolution — Resolver still
	// returns a string, just a relative one ("/cases/CASE-1" instead of
	// "https://.../cases/CASE-1") — which silently produces an email a
	// recipient can't actually click through on. Warn at startup for the
	// same reason as CUSTOMER_ENTITY_BASE_URL above: a misconfigured
	// deployment should find out now, not from a support ticket about a
	// broken link.
	if customerPortalBaseURL == "" {
		slog.Warn("CUSTOMER_PORTAL_WEB_BASE_URL is not set; recipients classified customer will get a relative (non-clickable) case link")
	}
	if csmPortalBaseURL == "" {
		slog.Warn("CSM_PORTAL_WEB_BASE_URL is not set; recipients classified CSM will get a relative (non-clickable) case link")
	}
	linkResolver := recipientlinks.New(customerEntityClient, recipientlinks.Config{
		CustomerRoles:   splitComma(os.Getenv("CUSTOMER_ROLES")),
		CSMRoles:        splitComma(os.Getenv("CSM_ROLES")),
		CustomerBaseURL: customerPortalBaseURL,
		CSMBaseURL:      csmPortalBaseURL,
	})

	// The event bus (Azure Event Hub's Kafka-compatible endpoint) is this
	// service's core purpose, unlike the notification channels above, so its
	// config is required (mustEnv) — a misconfigured deployment should fail
	// loudly at startup instead of silently accepting or dropping events.
	// This service is a pure consumer now: csm-portal-backend and
	// customer-portal-backend publish directly to eventBusCfg's topic
	// themselves (see internal/events's package doc) — there is no HTTP
	// ingest endpoint here anymore.
	eventBusCfg := eventbus.Config{
		Broker:           mustEnv("EVENT_HUB_BROKER"),
		ConnectionString: mustEnv("EVENT_HUB_CONNECTION_STRING"),
		Topic:            mustEnv("EVENT_HUB_TOPIC"),
	}
	// The dead-letter topic is a second, separate Event Hub in the same
	// namespace — also required, since a main consumer with nowhere to
	// dead-letter an exhausted record would otherwise silently drop it (see
	// eventbus.OnExhausted's doc comment).
	dlqCfg := eventbus.Config{
		Broker:           eventBusCfg.Broker,
		ConnectionString: eventBusCfg.ConnectionString,
		Topic:            mustEnv("EVENT_HUB_DLQ_TOPIC"),
	}

	dlqProducer := eventbus.NewProducer(dlqCfg)
	defer dlqProducer.Close()

	consumerGroup := envOrDefault("EVENT_HUB_CONSUMER_GROUP", "csm-notification-service")
	dlqConsumerGroup := envOrDefault("EVENT_HUB_DLQ_CONSUMER_GROUP", "csm-notification-service-dlq")
	mainConsumerCount := envInt("MAIN_CONSUMER_COUNT", 1)
	dlqConsumerCount := envInt("DLQ_CONSUMER_COUNT", 1)

	// EMAIL_DEBUG_MODE redirects the four case.* types' actual email delivery
	// to EMAIL_DEBUG_RECIPIENTS instead of each event's real resolved
	// recipients — unset/anything but "true" means real sending to real
	// recipients. Unlike a killswitch, debug mode still sends a real email
	// (see dispatch.Dispatcher.emailDebugMode's doc comment) — it's meant
	// for exercising a dev/staging deployment end-to-end without risking a
	// real mailbox. Doesn't affect Google Chat/Twilio.
	emailDebugMode := os.Getenv("EMAIL_DEBUG_MODE") == "true"
	emailDebugRecipients := splitComma(os.Getenv("EMAIL_DEBUG_RECIPIENTS"))
	if emailDebugMode {
		slog.Warn("EMAIL_DEBUG_MODE=true; case.* emails will be redirected to EMAIL_DEBUG_RECIPIENTS", "recipientCount", len(emailDebugRecipients))
	}

	// Temporary killswitch for case.* email sending entirely — checked
	// before EMAIL_DEBUG_MODE above, so it silences email regardless of
	// debug mode. Matches this repo's own AUTH_TOKEN_VALIDATOR_ENABLED
	// disable-entirely convention (apps/csm-portal/backend), the same as
	// CALL_SENDING_ENABLED below. Meant for temporarily silencing email
	// while investigating a delivery issue without also having to stop
	// exercising the rest of the pipeline (link resolution, Chat, Twilio).
	emailSendingEnabled := os.Getenv("EMAIL_SENDING_ENABLED") != "false"
	if !emailSendingEnabled {
		slog.Warn("EMAIL_SENDING_ENABLED=false; case.* emails will be logged, not sent")
	}

	// Temporary killswitch, matching this repo's own AUTH_TOKEN_VALIDATOR_ENABLED
	// convention (apps/csm-portal/backend), for incident.created's Twilio
	// call specifically — doesn't affect the Google Chat alert. Unlike
	// EMAIL_DEBUG_MODE above, calls have no debug-recipient equivalent, so
	// this keeps the simpler disable-entirely (log-only) shape.
	callSendingEnabled := os.Getenv("CALL_SENDING_ENABLED") != "false"
	if !callSendingEnabled {
		slog.Warn("CALL_SENDING_ENABLED=false; incident.created calls will be logged, not placed")
	}

	// Fallback Google Chat product (case.created and incident.created alike)
	// and on-call number (incident.created's call only) for when a publisher
	// (e.g. entity-service) can't determine which Chat space or on-call
	// number applies and omits them from the payload — see
	// dispatch.Dispatcher.defaultChatProduct/defaultOnCallNumber.
	defaultChatProduct := os.Getenv("DEFAULT_CHAT_PRODUCT")
	defaultOnCallNumber := os.Getenv("INCIDENT_DEFAULT_CALL_TO")

	dispatcher := dispatch.NewDispatcher(emailClient, googleChatClient, twilioClient, linkResolver, emailSendingEnabled, emailDebugMode, emailDebugRecipients, callSendingEnabled, defaultChatProduct, defaultOnCallNumber)

	// The main consumer's OnExhausted: publish the exhausted record to the
	// dead-letter topic instead of just logging and dropping it. The DLQ's
	// own consumer (started below) gets onExhausted=nil — there is
	// deliberately no third tier past the DLQ; see handleAttempts' doc
	// comment in eventbus/consumer.go.
	toDeadLetter := func(ctx context.Context, record eventbus.Record, handleErr error) error {
		slog.WarnContext(ctx, "eventbus: handler exhausted retries, publishing to dead-letter topic",
			"topic", record.Topic, "partition", record.Partition, "offset", record.Offset,
			"dlqTopic", dlqCfg.Topic, "err", handleErr)
		return dlqProducer.Publish(ctx, record.Key, record.Value)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	addr := ":" + mustPort("PORT", "8080")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("CSM Notification Service started", "addr", addr)

	// No Auth layer in this middleware chain — inbound requests are trusted at the
	// Choreo API Manager gateway (subscription + M2M app auth), not validated again
	// in this service. The only route left is /health — Choreo's own liveness
	// probe — since this service has no other inbound HTTP surface anymore.
	srv := &http.Server{
		Handler: middleware.SecurityHeaders(
			middleware.CorrelationID(
				middleware.Logger(mux),
			),
		),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	mainConsumers := startConsumers(ctx, "main", eventBusCfg, consumerGroup, mainConsumerCount, dispatcher.Handle, toDeadLetter)
	dlqConsumers := startConsumers(ctx, "dlq", dlqCfg, dlqConsumerGroup, dlqConsumerCount, dispatcher.Handle, nil)

	// The SLA timer engine is optional per deployment, gated on REDIS_ADDR or
	// REDIS_URL being set — unset means this engine neither consumes
	// sla.clock.register nor ticks, matching the "unset means don't run"
	// convention used elsewhere in this repo's own services for an optional
	// capability (e.g. apps/csm-portal/backend's EVENT_HUB_BROKER gate).
	// REDIS_URL (a rediss://:<password>@<host>:<port> connection string,
	// parsed via redis.ParseURL) is how a managed Redis with TLS — Azure
	// Managed Redis, Azure Cache for Redis — gets configured: the "rediss"
	// scheme makes go-redis dial with TLS automatically, which a plain
	// REDIS_ADDR/REDIS_PASSWORD pair has no way to request. REDIS_ADDR/
	// REDIS_PASSWORD remain for a local, non-TLS Redis and take effect only
	// when REDIS_URL is unset.
	//
	// redisClient below is always a plain redis.NewClient, which only
	// supports a non-clustered Redis (a single logical endpoint, whether
	// that's a real standalone instance or Azure Managed Redis/Azure Cache
	// for Redis under a non-clustered or "Enterprise" clustering policy,
	// where Azure's own proxy hides the sharding). It does NOT support
	// "OSS Cluster" policy — that needs a cluster-aware redis.NewClusterClient
	// to follow MOVED/ASK redirects, which nothing here constructs. Confirm
	// the target Redis resource's clustering policy is Enterprise/
	// non-clustered before pointing REDIS_URL at it; OSS Cluster policy will
	// fail unpredictably (WakeIndex's ZSET operations landing on the wrong
	// shard) rather than at this construction site.
	var redisClient *redis.Client
	var slaProducer *eventbus.Producer
	var slaConsumers []*eventbus.Consumer
	redisURL := os.Getenv("REDIS_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisURL != "" || redisAddr != "" {
		var redisOpts *redis.Options
		if redisURL != "" {
			var err error
			redisOpts, err = redis.ParseURL(redisURL)
			if err != nil {
				// Deliberately not logging err itself: a malformed URL (e.g.
				// a stray unescaped '%' in the password) makes Go's
				// net/url.Parse embed the raw input string — password
				// included — in its own error message, which would
				// otherwise land straight in this log line.
				slog.Error("invalid REDIS_URL: failed to parse connection string")
				os.Exit(1)
			}
		} else {
			redisOpts = &redis.Options{Addr: redisAddr, Password: os.Getenv("REDIS_PASSWORD")}
		}
		redisClient = redis.NewClient(redisOpts)

		// slaengine.EntityClient talks to the exact same entity-service as
		// customerEntityClient above — not a different backend — so it
		// reuses that same CUSTOMER_ENTITY_BASE_URL/CUSTOMER_ENTITY_SCOPES
		// pair (and the same shared OAuth2 app) rather than a redundant
		// SLA-specific one. It's still a separate client/type from
		// customerEntityClient, since internal/entity.CustomerEntityClient
		// deliberately implements only POST /users/search (see its own doc
		// comment) — not because the two point at different servers.
		//
		// Unlike customerEntityClient's own construction above (which reads
		// the OAuth2 triple with plain os.Getenv, since that feature only
		// warns-and-degrades on a missing config), mustEnv is used for all
		// four values here: once REDIS_ADDR opts into this engine, every one
		// of them is required for it to do anything at all — a missing
		// credential would otherwise silently fail every entity-service call
		// this engine makes, with each sla.clock.register record retried and
		// dead-lettered for a reason invisible from the DLQ topic alone.
		slaEntityClient := slaengine.NewEntityClient(slaengine.EntityConfig{
			BaseURL:      mustEnv("CUSTOMER_ENTITY_BASE_URL"),
			TokenURL:     mustEnv("OAUTH2_TOKEN_URL"),
			ClientID:     mustEnv("OAUTH2_CLIENT_ID"),
			ClientSecret: mustEnv("OAUTH2_CLIENT_SECRET"),
			Scopes:       splitComma(os.Getenv("CUSTOMER_ENTITY_SCOPES")),
		})

		// Reuses eventBusCfg's topic (the same one dispatcher's main consumer
		// reads) rather than a dedicated one — no new Azure Event Hub topic
		// needs provisioning for this feature; splitting sla.tier_reached
		// onto its own topic is a later call once a real consumer of it
		// exists.
		slaProducer = eventbus.NewProducer(eventBusCfg)

		slaEngine := slaengine.NewEngine(slaEntityClient, slaengine.NewWakeIndex(redisClient), slaProducer, googleChatClient, linkResolver, defaultChatProduct)

		slaConsumerGroup := envOrDefault("SLA_CONSUMER_GROUP", "csm-notification-service-sla")
		slaConsumerCount := envInt("SLA_CONSUMER_COUNT", 1)
		slaConsumers = startConsumers(ctx, "sla", eventBusCfg, slaConsumerGroup, slaConsumerCount, slaEngine.Handle, toDeadLetter)

		tickInterval := envDuration("SLA_TICK_INTERVAL", 15*time.Second)
		go slaEngine.RunTicker(ctx, tickInterval)
	}

	// timecardengine has no Redis/state dependency at all (unlike slaEngine
	// above) — it's a plain Kafka consumer, so it's started unconditionally,
	// not gated behind the REDIS_URL/REDIS_ADDR check. Its own dedicated
	// consumer group, not dispatcher's — see that package's own doc comment
	// for why. Its Handle is currently log-only (see the package doc
	// comment): entity-service's own Publish call for events.
	// TypeCaseBillableStatusChanged is itself still commented out, so this
	// consumer group exists ahead of having anything to actually do yet.
	timeCardEngine := timecardengine.NewEngine()
	timeCardConsumerGroup := envOrDefault("TIME_CARD_CONSUMER_GROUP", "csm-notification-service-time-card")
	timeCardConsumerCount := envInt("TIME_CARD_CONSUMER_COUNT", 1)
	timeCardConsumers := startConsumers(ctx, "time-card", eventBusCfg, timeCardConsumerGroup, timeCardConsumerCount, timeCardEngine.Handle, toDeadLetter)

	<-ctx.Done()
	stop()

	for _, c := range mainConsumers {
		c.Close()
	}
	for _, c := range dlqConsumers {
		c.Close()
	}
	for _, c := range slaConsumers {
		c.Close()
	}
	for _, c := range timeCardConsumers {
		c.Close()
	}
	if slaProducer != nil {
		slaProducer.Close()
	}
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			slog.Error("failed to close redis client", "err", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("CSM Notification Service stopped")
}

// startConsumers starts count independent eventbus.Consumer instances, all
// joining group and consuming cfg.Topic — Kafka's own consumer-group
// rebalancing splits cfg.Topic's partitions across however many of them are
// actually running, so count is a plain concurrency knob, not something this
// function has to implement partition assignment for itself. Each instance
// runs handle (and onExhausted, on retry exhaustion) in its own goroutine,
// sharing ctx for shutdown.
//
// Before starting anything, checks cfg.Topic's real partition count and logs
// a warning (never fails startup over this) if count exceeds it — a Kafka
// consumer group never hands out more partitions than exist, so a consumer
// count higher than the partition count just leaves the excess consumers
// permanently idle rather than doing anything actively wrong.
func startConsumers(ctx context.Context, name string, cfg eventbus.Config, group string, count int, handle eventbus.Handle, onExhausted eventbus.OnExhausted) []*eventbus.Consumer {
	if partitions, err := eventbus.PartitionCount(ctx, cfg); err != nil {
		slog.Warn("failed to check partition count; skipping the consumer-count sanity check", "consumer", name, "topic", cfg.Topic, "err", err)
	} else if count > partitions {
		slog.Warn("consumer count exceeds the topic's partition count; excess consumers will sit idle",
			"consumer", name, "topic", cfg.Topic, "consumerCount", count, "partitions", partitions)
	}

	consumers := make([]*eventbus.Consumer, count)
	for i := range consumers {
		c := eventbus.NewConsumer(cfg, group)
		consumers[i] = c
		go c.Run(ctx, handle, onExhausted)
	}
	slog.Info("consumer group started", "consumer", name, "topic", cfg.Topic, "consumerGroup", group, "count", count)
	return consumers
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

// envInt returns the given environment variable parsed as an int, or def if
// unset, malformed, or not positive — used for the MAIN_CONSUMER_COUNT/
// DLQ_CONSUMER_COUNT knobs, where zero or negative would be meaningless.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		slog.Warn("environment variable is not a valid positive integer; using default", "key", key, "value", v, "default", def)
		return def
	}
	return n
}

// envDuration returns the given environment variable parsed with
// time.ParseDuration (e.g. "15s", "2m"), or def if unset or malformed — used
// for SLA_TICK_INTERVAL, where an invalid value should fall back rather than
// fail startup, matching envInt's own default-on-malformed behavior.
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		// time.ParseDuration accepts "0s" and negative strings without
		// erroring — both would later panic time.NewTicker (its duration
		// must be > 0), so they're treated the same as a parse failure here.
		slog.Warn("environment variable is not a valid positive duration; using default", "key", key, "value", v, "default", def)
		return def
	}
	return d
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
		if _, present := os.LookupEnv(k); !present {
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

// parseGoogleChatSpaces decodes GOOGLE_CHAT_SPACES, a JSON array of
// {"product":"...","webhookUrl":"..."} objects — one per Google Chat space.
// A missing or malformed value logs a warning and yields no spaces rather
// than failing startup, since this channel is not required for every
// deployment.
func parseGoogleChatSpaces(raw string) []notifications.GoogleChatSpace {
	if raw == "" {
		return nil
	}
	var spaces []notifications.GoogleChatSpace
	if err := json.Unmarshal([]byte(raw), &spaces); err != nil {
		slog.Error("failed to parse GOOGLE_CHAT_SPACES; Google Chat alerts will be unavailable", "err", err)
		return nil
	}
	return spaces
}
