// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
// Package snsconfirm handles AWS SNS subscription confirmations the way the ServiceNow AWS Alert
// API did (AWSSNSNotificationUtils): confirm the subscription by fetching its SubscribeURL. No
// alert is stored. Unlike ServiceNow, the SNS signature is verified first; an unsigned or forged
// confirmation is ignored.
package snsconfirm

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// Handler confirms AWS SNS subscriptions.
type Handler struct {
	logger *slog.Logger
	http   *http.Client
	// allowURL guards the confirmation fetch; only SNS's own endpoints by default.
	allowURL func(*url.URL) bool
	verifier *verifier
}

// New returns a Handler.
func New(logger *slog.Logger, timeout time.Duration) *Handler {
	h := &Handler{logger: logger,
		http: &http.Client{Timeout: timeout, CheckRedirect: noRedirects}, allowURL: isSNSURL}
	h.verifier = &verifier{http: h.http, allowURL: func(u *url.URL) bool { return h.allowURL(u) }}
	return h
}

var snsHost = regexp.MustCompile(`^sns\.[a-z0-9-]+\.amazonaws\.com(\.cn)?$`)

// isSNSURL accepts only https URLs on an AWS SNS endpoint, so the service can't fetch arbitrary addresses.
func isSNSURL(u *url.URL) bool {
	return u.Scheme == "https" && snsHost.MatchString(u.Hostname())
}

// noRedirects keeps every fetch on the SNS host that was checked.
func noRedirects(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// HandleIfConfirmation handles raw if it is an SNS SubscriptionConfirmation and reports whether it was one.
func (h *Handler) HandleIfConfirmation(raw []byte) bool {
	var msg message
	if json.Unmarshal(raw, &msg) != nil || msg.Type != "SubscriptionConfirmation" {
		return false
	}
	if err := h.verifier.verify(msg); err != nil {
		h.logger.Warn("SNS subscription confirmation failed signature check; ignored",
			"topic_arn", msg.TopicArn, "error", err)
		return true
	}
	if msg.SubscribeURL == "" {
		h.logger.Error("SNS subscription confirmation has no SubscribeURL", "topic_arn", msg.TopicArn)
		return true
	}

	if u, err := url.Parse(msg.SubscribeURL); err != nil || !h.allowURL(u) {
		h.logger.Error("SNS SubscribeURL is not an AWS SNS https URL; not fetched",
			"topic_arn", msg.TopicArn, "subscribe_url", msg.SubscribeURL)
		return true
	}
	if !h.confirm(msg.SubscribeURL) {
		h.logger.Error("CRITICAL: SNS subscription failed auto-confirm; a manual ConfirmSubscription click is needed",
			"topic_arn", msg.TopicArn, "subscribe_url", msg.SubscribeURL)
	}
	return true
}

func (h *Handler) confirm(subscribeURL string) bool {
	u, err := url.Parse(subscribeURL)
	if err != nil || !h.allowURL(u) {
		h.logger.Error("SNS SubscribeURL is not an AWS SNS https URL; not fetched", "subscribe_url", subscribeURL)
		return false
	}
	req, err := http.NewRequest(http.MethodGet, subscribeURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "sre-alert-ingestion-service")
	resp, err := h.http.Do(req)
	if err != nil {
		h.logger.Error("SNS subscription auto-confirm request failed", "error", err)
		return false
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		h.logger.Error("SNS subscription auto-confirm rejected", "status", resp.StatusCode)
		return false
	}
	h.logger.Info("SNS subscription confirmed", "subscribe_host", u.Host)
	return true
}
