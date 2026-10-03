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
	"fmt"
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
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/dispatch"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/escalation"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/scim"
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
		ReplyTo:      splitComma(emailReplyTo()),
	})

	// Google Chat is likewise optional per deployment; a missing or malformed
	// value logs a warning and yields no spaces rather than failing startup.
	// GOOGLE_CHAT_SPACES is audience-keyed (team name, or a standing
	// audience like "Incident Monitor") — the only Chat routing config this
	// service has; there is no product-based alternative any more.
	if os.Getenv("GOOGLE_CHAT_AUDIENCE_SPACES") != "" {
		slog.Warn("GOOGLE_CHAT_AUDIENCE_SPACES is set but no longer read; it was renamed to GOOGLE_CHAT_SPACES, which is now the only Google Chat routing config")
	}
	googleChatClient := notifications.NewGoogleChatClient(notifications.GoogleChatConfig{
		AudienceSpaces: parseGoogleChatAudienceSpaces(os.Getenv("GOOGLE_CHAT_SPACES")),
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
		// Without this the field was documented, settable, and read by
		// nothing: every production ladder call rang for Twilio's 60s default
		// whatever an operator configured. It matters for a ladder because a
		// rung's next attempt can come due while the previous call is still
		// ringing.
		RingTimeoutSeconds: envInt("TWILIO_RING_TIMEOUT_SECONDS", 0),
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

	// The change-request notices ride their own topic, not case-events.
	// A consumer group reads its whole topic, so sharing one would make this
	// service's case consumer read and discard every change-request record
	// and the change-request consumer read and discard every case one --
	// a separate group isolates processing, only a separate topic isolates
	// volume. entity-service publishes here (CR_EVENT_HUB_TOPIC there).
	crCfg := eventbus.Config{
		Broker:           eventBusCfg.Broker,
		ConnectionString: eventBusCfg.ConnectionString,
		Topic:            envOrDefault("CR_EVENT_HUB_TOPIC", "cr-events"),
	}
	crDLQCfg := eventbus.Config{
		Broker:           eventBusCfg.Broker,
		ConnectionString: eventBusCfg.ConnectionString,
		Topic:            envOrDefault("CR_EVENT_HUB_DLQ_TOPIC", "cr-events-dlq"),
	}

	crDLQProducer := eventbus.NewProducer(crDLQCfg)
	defer crDLQProducer.Close()

	// The escalation ladder needs a dead-letter topic of its own, and for a
	// sharper reason than isolation.
	//
	// It shares the case topic with the dispatcher but runs a different
	// handler. Dead-lettering a ladder record onto the shared DLQ hands it to
	// the DLQ consumer, which runs dispatcher.Handle -- so an incident.created
	// whose LADDER exhausted its retries would be dispatched a second time,
	// placing another immediate call for an incident the dispatcher had
	// already handled, while the ladder itself was never retried at all. One
	// duplicate call, and still no escalation.
	escalationDLQCfg := eventbus.Config{
		Broker:           eventBusCfg.Broker,
		ConnectionString: eventBusCfg.ConnectionString,
		Topic:            envOrDefault("INCIDENT_ESCALATION_DLQ_TOPIC", "escalation-events-dlq"),
	}
	escalationDLQProducer := eventbus.NewProducer(escalationDLQCfg)
	defer escalationDLQProducer.Close()

	// The onboarding events ride their own topic too, for the same reason
	// the change-request notices do: a separate consumer group isolates
	// processing, only a separate topic isolates volume. An invitation
	// backlog must never sit behind a flood of case events, and the
	// onboarding dead-letter queue is watched on its own rather than mixed
	// into the case one. entity-service publishes project_contact.invited
	// here (PROJECT_EVENT_HUB_TOPIC there); every other type stays where
	// it is.
	projectCfg := eventbus.Config{
		Broker:           eventBusCfg.Broker,
		ConnectionString: eventBusCfg.ConnectionString,
		Topic:            envOrDefault("PROJECT_EVENT_HUB_TOPIC", "project-events"),
	}
	projectDLQCfg := eventbus.Config{
		Broker:           eventBusCfg.Broker,
		ConnectionString: eventBusCfg.ConnectionString,
		Topic:            envOrDefault("PROJECT_EVENT_HUB_DLQ_TOPIC", "project-events-dlq"),
	}

	projectDLQProducer := eventbus.NewProducer(projectDLQCfg)
	defer projectDLQProducer.Close()

	consumerGroup := envOrDefault("EVENT_HUB_CONSUMER_GROUP", "csm-notification-service")
	dlqConsumerGroup := envOrDefault("EVENT_HUB_DLQ_CONSUMER_GROUP", "csm-notification-service-dlq")
	mainConsumerCount := envInt("MAIN_CONSUMER_COUNT", 1)
	dlqConsumerCount := envInt("DLQ_CONSUMER_COUNT", 1)
	crConsumerGroup := envOrDefault("CR_CONSUMER_GROUP", "csm-notification-service-cr")
	crDLQConsumerGroup := envOrDefault("CR_DLQ_CONSUMER_GROUP", "csm-notification-service-cr-dlq")
	crConsumerCount := envInt("CR_CONSUMER_COUNT", 1)
	crDLQConsumerCount := envInt("CR_DLQ_CONSUMER_COUNT", 1)
	projectConsumerGroup := envOrDefault("PROJECT_CONSUMER_GROUP", "csm-notification-service-project")
	projectDLQConsumerGroup := envOrDefault("PROJECT_DLQ_CONSUMER_GROUP", "csm-notification-service-project-dlq")
	projectConsumerCount := envInt("PROJECT_CONSUMER_COUNT", 1)
	projectDLQConsumerCount := envInt("PROJECT_DLQ_CONSUMER_COUNT", 1)

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
	emailSendingEnabled := envBool("EMAIL_SENDING_ENABLED", true)
	// A customer-audience notice puts the recipients in BCC and uses the from
	// address as the only To, so an unset EMAIL_FROM_ADDRESS submits [""] to
	// the email service rather than failing here. Required whenever sending is
	// on, which every real deployment already satisfies.
	if emailSendingEnabled && strings.TrimSpace(os.Getenv("EMAIL_FROM_ADDRESS")) == "" {
		slog.Error("EMAIL_FROM_ADDRESS is required when EMAIL_SENDING_ENABLED is not \"false\"")
		os.Exit(1)
	}
	if !emailSendingEnabled {
		slog.Warn("EMAIL_SENDING_ENABLED=false; case.* emails will be logged, not sent")
	}

	// Temporary killswitch, matching this repo's own AUTH_TOKEN_VALIDATOR_ENABLED
	// convention (apps/csm-portal/backend), for incident.created's Twilio
	// call specifically — doesn't affect the Google Chat alert. Unlike
	// EMAIL_DEBUG_MODE above, calls have no debug-recipient equivalent, so
	// this keeps the simpler disable-entirely (log-only) shape.
	callSendingEnabled := envBool("CALL_SENDING_ENABLED", true)
	if !callSendingEnabled {
		slog.Warn("CALL_SENDING_ENABLED=false; incident.created calls will be logged, not placed")
	}

	// Fallback on-call number (incident.created's call only) for when a
	// publisher (e.g. entity-service) can't determine which on-call number
	// applies and omits it from the payload — see
	// dispatch.Dispatcher.defaultOnCallNumber.
	defaultOnCallNumber := os.Getenv("INCIDENT_DEFAULT_CALL_TO")

	// DEFAULT_CSM_EMAIL_CC is CC'd on every case.* email's CSM-portal-link
	// group only (never the customer-portal group, never during
	// EMAIL_DEBUG_MODE) — see dispatch.Dispatcher.defaultCSMEmailCC's own
	// doc comment.
	defaultCSMEmailCC := splitComma(os.Getenv("DEFAULT_CSM_EMAIL_CC"))

	dispatcher := dispatch.NewDispatcher(emailClient, googleChatClient, twilioClient, linkResolver, emailSendingEnabled, emailDebugMode, emailDebugRecipients, callSendingEnabled, defaultOnCallNumber, defaultCSMEmailCC).
		WithOnboarding(loadOnboardingConfig(customerEntityClient, emailClient))

	// The main consumer's OnExhausted: publish the exhausted record to the
	// dead-letter topic instead of just logging and dropping it. The DLQ's
	// own consumer (started below) gets onExhausted=nil — there is
	// deliberately no third tier past the DLQ; see handleAttempts' doc
	// comment in eventbus/consumer.go.
	toDeadLetter := func(ctx context.Context, record eventbus.Record, handleErr error) error {
		attrs := []any{"topic", record.Topic, "partition", record.Partition,
			"offset", record.Offset, "dlqTopic", dlqCfg.Topic}
		slog.WarnContext(ctx, "eventbus: handler exhausted retries, publishing to dead-letter topic",
			append(attrs, deadLetterErrAttrs(handleErr)...)...)
		return dlqProducer.Publish(ctx, record.Key, record.Value)
	}

	// The change-request consumer dead-letters to its own topic, so a stuck
	// change-request record cannot fill the case DLQ (and the reverse).
	crToDeadLetter := func(ctx context.Context, record eventbus.Record, handleErr error) error {
		attrs := []any{"topic", record.Topic, "partition", record.Partition,
			"offset", record.Offset, "dlqTopic", crDLQCfg.Topic}
		slog.WarnContext(ctx, "eventbus: handler exhausted retries, publishing to dead-letter topic",
			append(attrs, deadLetterErrAttrs(handleErr)...)...)
		return crDLQProducer.Publish(ctx, record.Key, record.Value)
	}

	// The escalation ladder's own, for the reason escalationDLQCfg gives: a
	// record retried by the wrong handler is a duplicate call.
	escalationToDeadLetter := func(ctx context.Context, record eventbus.Record, handleErr error) error {
		attrs := []any{"topic", record.Topic, "partition", record.Partition,
			"offset", record.Offset, "dlqTopic", escalationDLQCfg.Topic}
		slog.WarnContext(ctx, "eventbus: escalation handler exhausted retries, publishing to its own dead-letter topic",
			append(attrs, deadLetterErrAttrs(handleErr)...)...)
		return escalationDLQProducer.Publish(ctx, record.Key, record.Value)
	}

	// Same again for the onboarding consumer: a stuck invitation cannot
	// fill the case or change-request DLQ, and the reverse.
	projectToDeadLetter := func(ctx context.Context, record eventbus.Record, handleErr error) error {
		attrs := []any{"topic", record.Topic, "partition", record.Partition,
			"offset", record.Offset, "dlqTopic", projectDLQCfg.Topic}
		slog.WarnContext(ctx, "eventbus: handler exhausted retries, publishing to dead-letter topic",
			append(attrs, deadLetterErrAttrs(handleErr)...)...)
		return projectDLQProducer.Publish(ctx, record.Key, record.Value)
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
	// Same dispatcher as the case consumers: it already routes on the
	// envelope's Type, and these two only ever receive change_request.* since
	// that is all their topic carries.
	crConsumers := startConsumers(ctx, "cr", crCfg, crConsumerGroup, crConsumerCount, dispatcher.Handle, crToDeadLetter)
	crDLQConsumers := startConsumers(ctx, "cr-dlq", crDLQCfg, crDLQConsumerGroup, crDLQConsumerCount, dispatcher.Handle, nil)
	// And the same for project_contact.invited: the one dispatcher routes
	// on the envelope's Type already, and these two only ever receive the
	// onboarding events since that is all their topic carries.
	projectConsumers := startConsumers(ctx, "project", projectCfg, projectConsumerGroup, projectConsumerCount, dispatcher.Handle, projectToDeadLetter)
	projectDLQConsumers := startConsumers(ctx, "project-dlq", projectDLQCfg, projectDLQConsumerGroup, projectDLQConsumerCount, dispatcher.Handle, nil)

	// The SLA breach-alerting engine is optional per deployment, gated on
	// REDIS_ADDR or REDIS_URL being set — unset means this engine never
	// polls, matching the "unset means don't run" convention used elsewhere
	// in this repo's own services for an optional capability (e.g.
	// apps/csm-portal/backend's EVENT_HUB_BROKER gate). Unlike the design
	// this replaced, it is no longer a Kafka consumer at all — see
	// internal/slaengine's own CLAUDE.md section ("SLA breach alerting")
	// for the full redesign: it polls entity-service's GET /sla-status
	// (backed by the real, ServiceNow-synced "sla" table, not a value this
	// service used to compute itself) on a plain ticker instead.
	//
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
	// fail unpredictably (TierStore's key operations landing on the wrong
	// shard) rather than at this construction site.
	var redisClient *redis.Client
	var slaProducer *eventbus.Producer
	var escalationConsumers []*eventbus.Consumer
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
		// credential would otherwise silently fail every poll.
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

		slaEngine := slaengine.NewEngine(slaEntityClient, slaengine.NewTierStore(redisClient), slaProducer, googleChatClient, linkResolver, emailClient, emailSendingEnabled, emailDebugMode, emailDebugRecipients)

		// SLA_TICK_INTERVAL defaults far above the old wake-index engine's
		// 15s: that interval made sense for firing a precomputed due date
		// close to when it actually elapsed, but this engine now polls
		// entity-service directly every tick (paginating through every
		// active clock, ~5,500 as of this redesign) and only needs to
		// notice a newly-crossed 50/75/100% checkpoint, not a specific
		// instant — most active "sla" rows don't change more than a few
		// times a day. 5 minutes balances alert latency against load on
		// entity-service and Redis.
		tickInterval := envDuration("SLA_TICK_INTERVAL", 5*time.Minute)
		go slaEngine.RunTicker(ctx, tickInterval)

		// The incident call-escalation ladder (internal/escalation) shares
		// this same Redis — its own keys, its own ZSET — and its own consumer
		// group on the same topic, exactly as the SLA engine does. It is
		// nested inside the Redis block for the same reason: without durable
		// state a ladder would forget everything it had scheduled on the
		// first restart, mid-page.
		//
		// It needs one more thing than Redis, though: a roster to resolve
		// levels to people (see escalation.RosterResolver for why that is
		// configuration rather than a ServiceNow lookup today). With none
		// configured, the engine is deliberately NOT started — a running
		// ladder that can never call anyone is worse than an absent one,
		// because it looks like coverage.
		roster, err := escalation.ParseRoster(os.Getenv("INCIDENT_ESCALATION_ROSTER"))
		escalationChannel, channelErr := escalation.ParseChannel(os.Getenv("INCIDENT_ESCALATION_CHANNEL"))

		// The configuration file governs behaviour; the environment still
		// holds the secrets. With no file, every knob keeps its previous
		// env-derived value, so an existing deployment behaves exactly as it
		// did -- see loadEscalationConfig.
		escalationCfg, cfgErr := loadEscalationConfig(escalationChannel)
		creCfg, escalationRunning := escalationCfg.For(escalation.LadderKeyCRE)
		if cfgErr == nil {
			// The file wins over INCIDENT_ESCALATION_CHANNEL when there is
			// one, so there is a single answer to "what will this dial".
			escalationChannel = creCfg.Channel
		}

		usingTeamSchedule := os.Getenv("INCIDENT_ESCALATION_RESOLVER") == "team-schedule"
		startProblem := escalationStartProblem(err != nil, roster.IsEmpty(),
			usingTeamSchedule, os.Getenv("CUSTOMER_ENTITY_BASE_URL") != "")
		// A Team Schedule resolver with no teams configured can resolve almost
		// nothing: no ABT keys and no ABT type means isABT is always false,
		// the team-lead and nominee rungs return nobody, and only the two
		// heads are reachable. The ladder would start, climb, and page almost
		// no one -- which is the "worse than an absent one, because it looks
		// like coverage" case escalationStartProblem exists to prevent, just
		// arriving through configuration rather than through a missing roster.
		if startProblem == "" && usingTeamSchedule &&
			len(creCfg.Teams.ABTs) == 0 && creCfg.Teams.ABTType == "" {
			startProblem = "INCIDENT_ESCALATION_RESOLVER=team-schedule needs teams.abtType or teams.abts " +
				"in the escalation configuration; without them almost every rung resolves to nobody"
		}
		switch {
		case cfgErr != nil:
			// Never a fall back to defaults: this file decides what gets
			// dialled, so a broken one means nothing runs until it is fixed.
			slog.Error("invalid escalation configuration; both ladders are disabled", "err", cfgErr)
		case !escalationRunning:
			slog.Warn("incident call escalation is disabled by configuration",
				"configPath", os.Getenv("INCIDENT_ESCALATION_CONFIG"))
		case startProblem != "":
			// Not logging a roster decode error itself: it can quote the
			// surrounding JSON, which carries real phone numbers.
			slog.Error(startProblem + "; incident call escalation is disabled")
		case channelErr != nil:
			slog.Error("invalid INCIDENT_ESCALATION_CHANNEL; incident call escalation is disabled", "err", channelErr)
		default:
			// Same entity-service and same shared OAuth2 app as the SLA
			// engine's client above — a separate client only because this one
			// speaks to /incidents rather than the sla_clocks endpoints.
			//
			// Unlike that one, this is os.Getenv and genuinely optional. The
			// SLA engine can do nothing at all without entity-service — its
			// clocks live there. This engine's job is notifying people; the
			// execution summary is a record of what it did. A deployment (or
			// a laptop) without entity-service access should still be able to
			// run a real ladder, with the summary logged instead of written
			// back — see Engine.writeNote's nil handling.
			var escalationNotes *escalation.EntityClient
			if base := os.Getenv("CUSTOMER_ENTITY_BASE_URL"); base != "" {
				escalationNotes = escalation.NewEntityClient(escalation.EntityConfig{
					BaseURL:      base,
					TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
					ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
					ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
					Scopes:       splitComma(os.Getenv("CUSTOMER_ENTITY_SCOPES")),
				})
			} else {
				slog.Warn("CUSTOMER_ENTITY_BASE_URL is not set; incident escalation will log its " +
					"execution summary instead of writing it back to the incident")
			}

			// Who each rung reaches. The Team Schedule is the real answer -
			// rota and rank, kept current by the people who own them - and the
			// hand-maintained roster is the stopgap it replaces. Reading the
			// schedule needs entity-service, so a deployment without it falls
			// back rather than starting with a resolver that cannot answer.
			//
			// Off by default: the rung model the schedule resolver implements
			// is still an assumption awaiting confirmation, and a deployment
			// should opt into it knowingly rather than inherit it on upgrade.
			var escalationResolver escalation.Resolver = escalation.NewRosterResolver(roster)
			if os.Getenv("INCIDENT_ESCALATION_RESOLVER") == "team-schedule" {
				if escalationNotes == nil {
					slog.Error("INCIDENT_ESCALATION_RESOLVER=team-schedule needs CUSTOMER_ENTITY_BASE_URL; " +
						"falling back to the configured roster")
				} else {
					escalationResolver = escalation.NewTeamScheduleResolver(
						escalationNotes, creCfg.Teams, creCfg.Rules).
						// The heads are two named people, not a team lookup.
						WithHeads(creCfg.Heads).
						// How the nominated rungs are read: which tiers exist,
						// and how many nominees a rung takes per team.
						WithAlertDuty(creCfg.AlertTiers(), creCfg.AlertDuty.PerTeam).
						// Who has gone longest without a call, for the evening
						// pairing's second call.
						WithCallHistory(escalation.NewStore(redisClient))
					slog.Info("incident escalation resolves rungs from the Team Schedule")
				}
			}

			escalationEngine := escalation.NewEngine(
				escalation.DefaultPolicy,
				escalationResolver,
				twilioClient,
				googleChatClient,
				// The ladder builds its own incident link. recipientlinks
				// lost IncidentLink when incident.created stopped posting a
				// Chat alert and nothing else needed one; a rung's card still
				// has to say where to go and look.
				escalation.PortalLinks{CSMBaseURL: csmPortalBaseURL},
				escalation.NewStore(redisClient),
				escalationNotes,
				// The audience a rung's card posts to when the incident names
				// no product of its own. Its own variable rather than the
				// dispatcher's old DEFAULT_CHAT_PRODUCT, which went away with
				// the incident Chat alert -- the ladder's room is its own
				// decision now.
				os.Getenv("INCIDENT_ESCALATION_CHAT_AUDIENCE"),
				escalation.EngineConfig{
					// Shares CALL_SENDING_ENABLED with
					// dispatch.handleIncidentCreated's single call: both are
					// the same outbound channel to the same people, and
					// splitting them would let a deployment silence one and
					// not the other.
					CallSendingEnabled: callSendingEnabled,
					// SSML is opt-in rather than the default: it changes how
					// every escalation call sounds, so a deployment should
					// hear it (escalation-local --speak) before switching.
					UseSSML: os.Getenv("INCIDENT_ESCALATION_SSML") == "true",
					// call, chat or both. Defaults to call — see
					// ParseChannel for why silently downgrading a pager to a
					// chat message would be the wrong default.
					Channel: escalationChannel,
					// Which incidents get a ladder, and what one may spend.
					Ladder: creCfg,
				},
			)

			escalationGroup := envOrDefault("INCIDENT_ESCALATION_CONSUMER_GROUP", "csm-notification-service-escalation")
			escalationCount := envInt("INCIDENT_ESCALATION_CONSUMER_COUNT", 1)
			escalationConsumers = startConsumers(ctx, "escalation", eventBusCfg, escalationGroup, escalationCount, escalationEngine.Handle, escalationToDeadLetter)

			// And a consumer for that DLQ running the LADDER's handler, so a
			// dead-lettered ladder gets its own retry pass rather than being
			// re-dispatched by the wrong one. onExhausted is nil: a record
			// that fails here too is logged and dropped, the same single extra
			// tier every other consumer here gets.
			escalationDLQGroup := envOrDefault("INCIDENT_ESCALATION_DLQ_CONSUMER_GROUP",
				"csm-notification-service-escalation-dlq")
			escalationDLQCount := envInt("INCIDENT_ESCALATION_DLQ_CONSUMER_COUNT", 1)
			escalationConsumers = append(escalationConsumers,
				startConsumers(ctx, "escalation-dlq", escalationDLQCfg, escalationDLQGroup,
					escalationDLQCount, escalationEngine.Handle, nil)...)

			// Ticks faster than the SLA engine's 15s: the shortest gap
			// between two calls in section 7.0's table is one minute (P0), so
			// a coarse tick would visibly smear a P0 ladder.
			escalationTick := envDuration("INCIDENT_ESCALATION_TICK_INTERVAL", 5*time.Second)
			go escalationEngine.RunTicker(ctx, escalationTick)

			slog.Info("incident call escalation is enabled",
				"channel", string(escalationChannel),
				"ssml", os.Getenv("INCIDENT_ESCALATION_SSML") == "true",
				"sending", callSendingEnabled)

			// dispatch.handleIncidentCreated's own single, immediate call to
			// INCIDENT_DEFAULT_CALL_TO predates the ladder and is NOT part of
			// the escalation specification — section 3.0's initial reaction
			// to a new incident is the Chat alert and an email, with calls
			// starting only after the priority's initial wait. Left in place
			// rather than removed, because a deployment with no roster still
			// relies on it as its only page; unset INCIDENT_DEFAULT_CALL_TO
			// to retire it once the ladder covers an environment. Logged so
			// the overlap is visible at startup rather than discovered by
			// being called twice.
			if defaultOnCallNumber != "" {
				slog.Warn("incident call escalation is enabled while INCIDENT_DEFAULT_CALL_TO is also set; " +
					"a new incident will get both the single immediate call and the escalation ladder")
			}
		}
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
	for _, c := range crConsumers {
		c.Close()
	}
	for _, c := range crDLQConsumers {
		c.Close()
	}
	for _, c := range projectConsumers {
		c.Close()
	}
	for _, c := range projectDLQConsumers {
		c.Close()
	}
	for _, c := range escalationConsumers {
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

// loadOnboardingConfig wires the project_contact.invited handler (the
// customer onboarding flow's identity + invitation-email steps — see
// dispatch.OnboardingConfig). Both steps are behind their own opt-in flag,
// CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED / CSM_MIGRATION_ONBOARD_EMAIL_ENABLED (`== "true"`, default
// off — the opt-in convention EMAIL_DEBUG_MODE uses, since shipping this
// dark is the point), so a deployment without them records both steps as
// SKIPPED on entity-service's ledger and does nothing else.
//
// The SCIM operations service client authenticates with the same shared
// OAUTH2_* app as the email and entity clients above (the deployment it
// points at goes through the same gateway app, scoped via SCIM_SCOPES) —
// mirroring apps/csm-portal/backend's own SCIM client — so only
// SCIM_BASE_URL/SCIM_SCOPES are its own. os.Getenv, not mustEnv, like every
// other optional client here; a missing SCIM_BASE_URL with the identity
// flag on is warned about at startup rather than discovered on the first
// invitation.
//
// The invitation goes out through its own notifications.EmailClient —
// same email service and credentials as emailClient, but bound to
// ONBOARD_EMAIL_FROM (falling back to EMAIL_FROM_ADDRESS), since
// EmailClient fixes its From at construction and the invitation may need a
// different sender than the case.* emails. Step recording reuses
// customerEntityClient (entity.CustomerEntityClient.RecordOnboardingStep)
// — the same entity-service, same shared app; entity-service additionally
// requires this service's OAuth2 client id to be in its
// AUTH_INTERNAL_CLIENT_IDS for that endpoint.
func loadOnboardingConfig(steps *entity.CustomerEntityClient, emailClient *notifications.EmailClient) dispatch.OnboardingConfig {
	// CSM_MIGRATION_* flags are opt-in: off unless exactly "true".
	identityEnabled := envBool("CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED", false)
	emailEnabled := envBool("CSM_MIGRATION_ONBOARD_EMAIL_ENABLED", false)

	scimBaseURL := os.Getenv("SCIM_BASE_URL")
	if identityEnabled && scimBaseURL == "" {
		slog.Warn("CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED=true but SCIM_BASE_URL is not set; project_contact.invited identity steps will fail until it is configured")
	}
	scimClient := scim.NewClient(scim.Config{
		BaseURL:      scimBaseURL,
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("SCIM_SCOPES")),
	})

	// The invitation reuses the main email client (same grant, same token
	// cache) and only overrides the sender when ONBOARD_EMAIL_FROM is set.
	emailFrom := strings.TrimSpace(os.Getenv("ONBOARD_EMAIL_FROM"))

	portalURL := strings.TrimRight(envOrDefault("ONBOARD_PORTAL_URL", "https://support.wso2.com"), "/")

	if identityEnabled || emailEnabled {
		slog.Info("customer onboarding steps enabled for project_contact.invited", "identity", identityEnabled, "email", emailEnabled, "portalUrl", portalURL)
	}
	return dispatch.OnboardingConfig{
		Identity:        scimClient,
		Email:           emailClient,
		Steps:           steps,
		IdentityEnabled: identityEnabled,
		EmailEnabled:    emailEnabled,
		PortalURL:       portalURL,
		EmailFrom:       emailFrom,
		ReplyTo:         emailClient.ReplyTo(),
	}
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

// envBool reads a boolean flag with its default spelled out at the call
// site. Only the literal strings "true" and "false" (after trimming) change
// the value; anything else, including unset, yields def. Killswitches such
// as EMAIL_SENDING_ENABLED default to true; every CSM_MIGRATION_* flag
// defaults to false and is turned on deliberately at cutover.
func envBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true":
		return true
	case "false":
		return false
	default:
		return def
	}
}

// emailReplyTo reads EMAIL_REPLY_TO: unset means support@wso2.com, set but
// empty means no Reply-To.
func emailReplyTo() string {
	if v, ok := os.LookupEnv("EMAIL_REPLY_TO"); ok {
		return v
	}
	return "support@wso2.com"
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

// parseGoogleChatAudienceSpaces decodes GOOGLE_CHAT_SPACES, a JSON array of
// {"audience":"...","webhookUrl":"..."} objects — one per Chat audience (a
// team's own space, or a standing audience like "Incident Monitor"; see
// internal/chataudience). This is the only Google Chat routing config this
// service has — there is no product-based alternative. A missing or
// malformed value logs a warning and yields no spaces rather than failing
// startup, since this channel is not required for every deployment.
//
// Decodes with DisallowUnknownFields specifically so a deployment that
// still carries the old product-keyed shape ({"product","webhookUrl"})
// fails loudly here instead of silently: a plain json.Unmarshal would
// ignore the unknown "product" key, decode every entry with an empty
// Audience, and NewGoogleChatClient would then silently drop every one of
// them (see its own doc comment) — dropping every Chat alert with nothing
// but a per-send warning, and no indication at startup that the rename
// needs a config change. An entry that decodes fine but still has an empty
// audience (e.g. a hand-edited config missing the field) gets its own
// explicit error log for the same reason.
func parseGoogleChatAudienceSpaces(raw string) []notifications.GoogleChatAudienceSpace {
	if raw == "" {
		return nil
	}
	var spaces []notifications.GoogleChatAudienceSpace
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spaces); err != nil {
		slog.Error("failed to parse GOOGLE_CHAT_SPACES; Google Chat alerts will be unavailable", "err", err)
		return nil
	}
	for i, s := range spaces {
		if strings.TrimSpace(s.Audience) == "" {
			slog.Error("GOOGLE_CHAT_SPACES entry has no audience and will be skipped", "index", i)
		}
	}
	return spaces
}

// deadLetterErrAttrs describes a handler failure without reproducing it. An
// upstream failure arrives as *apierror.Error, whose Error() embeds up to 256
// bytes of the response body -- which can carry recipient addresses or other
// content this service is not allowed to log (see CLAUDE.md). The status code
// and error type are enough to find the failure upstream.
func deadLetterErrAttrs(err error) []any {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		return []any{"errKind", "upstream", "status", apiErr.StatusCode}
	}
	return []any{"errKind", fmt.Sprintf("%T", err)}
}

// escalationStartProblem says why the escalation engine must not start, or ""
// when it can: it needs somebody to resolve rungs from. That is the Team
// Schedule when INCIDENT_ESCALATION_RESOLVER=team-schedule and entity-service
// is configured, and the INCIDENT_ESCALATION_ROSTER otherwise -- the roster is
// only required when it is what the ladder would actually read. A ladder that
// can never call anyone is worse than none, because it looks like coverage.
//
// A roster that is set but does not parse always stops the engine, in either
// mode: it is a configuration mistake, and starting anyway would hide it.
func escalationStartProblem(rosterInvalid, rosterEmpty, teamSchedule, entityConfigured bool) string {
	switch {
	case rosterInvalid:
		return "invalid INCIDENT_ESCALATION_ROSTER: failed to parse"
	case teamSchedule && entityConfigured:
		return ""
	case teamSchedule && rosterEmpty:
		return "INCIDENT_ESCALATION_RESOLVER=team-schedule needs CUSTOMER_ENTITY_BASE_URL, and there is no INCIDENT_ESCALATION_ROSTER to fall back to"
	case rosterEmpty:
		return "INCIDENT_ESCALATION_ROSTER is not set"
	}
	return ""
}

// loadEscalationConfig reads the escalation configuration file, or synthesises
// the pre-file behaviour when no path is set.
//
// The fallback is what keeps this change safe to deploy: a service with no
// INCIDENT_ESCALATION_CONFIG behaves exactly as it did before the file
// existed -- enabled, on whatever INCIDENT_ESCALATION_CHANNEL said, with no
// trigger conditions and no spending caps. A deployment adopts the file when
// it wants to narrow any of that, not because it was forced to all at once.
//
// INCIDENT_ESCALATION_ENABLED overrides the file's own master switch in both
// directions, so an operator can stop every ladder by setting one variable,
// without editing and shipping a file in the middle of an incident.
func loadEscalationConfig(envChannel escalation.Channel) (escalation.Config, error) {
	path := os.Getenv("INCIDENT_ESCALATION_CONFIG")

	cfg := escalation.Config{
		Enabled: true,
		CRE:     escalation.LadderConfig{Enabled: true, Channel: envChannel},
		SRE:     escalation.LadderConfig{Enabled: true, Channel: envChannel},
	}
	if path != "" {
		loaded, err := escalation.LoadConfig(path)
		if err != nil {
			return loaded, err
		}
		cfg = loaded
		slog.Info("escalation configuration loaded",
			"configPath", path, "enabled", cfg.Enabled,
			"creChannel", string(cfg.CRE.Channel), "sreChannel", string(cfg.SRE.Channel))
	}

	if raw := os.Getenv("INCIDENT_ESCALATION_ENABLED"); raw != "" {
		on := raw == "true"
		if on != cfg.Enabled {
			slog.Warn("INCIDENT_ESCALATION_ENABLED overrides the configuration file's master switch",
				"enabled", on, "fileSaid", cfg.Enabled)
		}
		cfg.Enabled = on
	}
	return cfg, nil
}
