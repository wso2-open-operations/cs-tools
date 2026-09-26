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
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/kbclient"
"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/kbembeddingengine"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/kbdraftengine"
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

	// Fallback Google Chat product (case.created and incident.created alike)
	// and on-call number (incident.created's call only) for when a publisher
	// (e.g. entity-service) can't determine which Chat space or on-call
	// number applies and omits them from the payload — see
	// dispatch.Dispatcher.defaultChatProduct/defaultOnCallNumber.
	defaultChatProduct := os.Getenv("DEFAULT_CHAT_PRODUCT")
	defaultOnCallNumber := os.Getenv("INCIDENT_DEFAULT_CALL_TO")

	dispatcher := dispatch.NewDispatcher(emailClient, googleChatClient, twilioClient, linkResolver, emailSendingEnabled, emailDebugMode, emailDebugRecipients, callSendingEnabled, defaultChatProduct, defaultOnCallNumber).
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

		slaEngine := slaengine.NewEngine(slaEntityClient, slaengine.NewTierStore(redisClient), slaProducer, googleChatClient, linkResolver, defaultChatProduct)

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

	// kbdraftengine (Flow 1 of the KB auto-generation work): reuses the
	// EXISTING case.status_changed event and case-events topic -- no new
	// topic needed for this flow. One shared consumer group for both new
	// KB flows (this one, and the future publish->embedding flow) --
	// see the KB-Auto-Generation-NOVA-Integration-Plan doc for the
	// reasoning (both flows are low-frequency relative to the main
	// dispatcher's traffic, so isolating them from EACH OTHER the way
	// SLA/time-card are isolated from the main dispatcher isn't
	// warranted the same way).
	//
	// KB_DRAFT_AUTHOR_ID and the knowledge-base resolution below are both
	// still open questions -- see kbdraftengine's own package doc.
	// MockDraftGenerator stands in for a real OpenAI call until a
	// test/placeholder key is available.
	kbEntityClient := kbclient.New(kbclient.Config{
		BaseURL:      os.Getenv("CUSTOMER_ENTITY_BASE_URL"),
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("CUSTOMER_ENTITY_SCOPES")),
	})
	kbDraftAuthorID := os.Getenv("KB_DRAFT_AUTHOR_ID")
	if kbDraftAuthorID == "" {
		slog.Warn("KB_DRAFT_AUTHOR_ID is not set; kbdraftengine will fail to save any draft until it is configured")
	}
	// TODO: placeholder resolution -- picks the first active knowledge
	// base rather than mapping the case's actual product. Replace once
	// the real case->KB mapping is confirmed (see kbdraftengine's doc).
	resolveKnowledgeBaseID := func(ctx context.Context, kb *kbclient.Client, caseID string) (string, error) {
		kbs, err := kb.ListKnowledgeBases(ctx)
		if err != nil {
			return "", err
		}
		for _, k := range kbs {
			if k.Active {
				return k.ID, nil
			}
		}
		return "", fmt.Errorf("no active knowledge base found")
	}
	var kbDraftGenerator kbdraftengine.DraftGenerator = kbdraftengine.MockDraftGenerator{}
	if azureKey := os.Getenv("AZURE_US_OPENAI_API_KEY"); azureKey != "" {
		kbDraftGenerator = kbdraftengine.NewAzureOpenAIDraftGenerator(
			os.Getenv("AZURE_US_OPENAI_ENDPOINT"),
			azureKey,
			os.Getenv("AZURE_US_OPENAI_DEPLOYMENT_NAME"), // TODO: EU routing not yet implemented, hardcoded to US for testing
		)
	}
	kbDraftEngine := kbdraftengine.New(kbEntityClient, kbDraftGenerator, kbDraftAuthorID, resolveKnowledgeBaseID)

	var kbEmbeddingGenerator kbembeddingengine.EmbeddingGenerator = kbembeddingengine.MockEmbeddingGenerator{}
	if embURL := os.Getenv("AZURE_US_OPENAI_EMBEDDING_URL"); embURL != "" {
		kbEmbeddingGenerator = kbembeddingengine.NewAzureOpenAIEmbeddingGenerator(
			embURL,
			os.Getenv("AZURE_US_OPENAI_EMBEDDING_KEY"),
			os.Getenv("AZURE_US_OPENAI_ENDPOINT"),
			os.Getenv("AZURE_US_OPENAI_API_KEY"),
			os.Getenv("AZURE_US_OPENAI_DEPLOYMENT_NAME"),
		)
	}
	var kbPineconeUpserter kbembeddingengine.PineconeUpserter = kbembeddingengine.MockPineconeUpserter{}
	if pineconeHost := os.Getenv("PINECONE_HOST"); pineconeHost != "" {
		kbPineconeUpserter = kbembeddingengine.NewPineconeClient(pineconeHost, os.Getenv("PINECONE_KEY"))
	}
	kbEmbeddingEngine := kbembeddingengine.New(kbEntityClient, kbEmbeddingGenerator, kbPineconeUpserter)

	kbCombinedHandle := func(ctx context.Context, record eventbus.Record) error {
		if err := kbDraftEngine.Handle(ctx, record); err != nil {
			return err
		}
		return kbEmbeddingEngine.Handle(ctx, record)
	}
	kbDraftConsumerGroup := envOrDefault("KB_DRAFT_CONSUMER_GROUP", "csm-notification-service-kb-embedding")
	kbDraftConsumerCount := envInt("KB_DRAFT_CONSUMER_COUNT", 1)
	kbDraftConsumers := startConsumers(ctx, "kb-draft", eventBusCfg, kbDraftConsumerGroup, kbDraftConsumerCount, kbCombinedHandle, toDeadLetter)

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
	for _, c := range timeCardConsumers {
		c.Close()
	}
	for _, c := range kbDraftConsumers {
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
