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

// Package emaildomains classifies a signup's email domain (POST
// /ai/email-domains/classify). Rules decide first: WSO2's domains, the
// disposable list, free providers, and a domain that does not exist.
// The model is asked only about the rest, sees only the domain, and its
// answers are cached by domain for a day.
package emaildomains

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// Classification is a portal option code of REGISTERED_EMAIL_CLASSIFICATION.
type Classification string

const (
	Corporate  Classification = "CORPORATE"
	Personal   Classification = "PERSONAL"
	Disposable Classification = "DISPOSABLE"
	Internal   Classification = "INTERNAL"
	Unknown    Classification = "UNKNOWN"
)

// Source says what decided the classification.
type Source string

const (
	SourceRules Source = "rules"
	SourceCache Source = "cache"
	SourceModel Source = "model"
)

const (
	cacheTTL  = 24 * time.Hour
	maxTokens = 256
)

// Reasons a domain gets no classification. The caller retries later.
var (
	ErrInvalidDomain = errors.New("not a valid domain")
	ErrNotConfigured = errors.New("model calls are not configured")
	ErrCall          = errors.New("model call failed")
	ErrUsageLimit    = errors.New("model usage limit reached")
	ErrRefused       = errors.New("model refused the request")
	ErrTruncated     = errors.New("model reply was cut off")
	ErrUnusable      = errors.New("model reply is not usable")
)

var domainRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,61}[a-z0-9]$`)

// Result is one domain's classification.
type Result struct {
	Domain         string
	Classification Classification
	Source         Source
	InputTokens    int64
	OutputTokens   int64
}

// Resolver is the subset of *net.Resolver used here.
type Resolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// Classifier runs the rules, then the model.
type Classifier struct {
	lists     *Lists
	resolver  Resolver
	mxTimeout time.Duration
	cache     *cache
	client    *anthropic.Client // nil when model calls are off
	model     string
	timeout   time.Duration
}

// NewClassifier returns a classifier. A nil client answers ErrNotConfigured
// for a domain the rules leave open.
func NewClassifier(lists *Lists, resolver Resolver, mxTimeout time.Duration, client *anthropic.Client, model string, timeout time.Duration) *Classifier {
	return &Classifier{lists: lists, resolver: resolver, mxTimeout: mxTimeout, cache: newCache(cacheTTL),
		client: client, model: model, timeout: timeout}
}

// Normalize lower-cases and checks a domain.
func Normalize(raw string) (string, error) {
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if len(d) > 253 || !domainRe.MatchString(d) {
		return "", ErrInvalidDomain
	}
	return d, nil
}

// Classify classifies one domain.
func (c *Classifier) Classify(ctx context.Context, raw string) (Result, error) {
	domain, err := Normalize(raw)
	if err != nil {
		return Result{}, err
	}
	res := Result{Domain: domain, Source: SourceRules}
	switch {
	case isInternal(domain):
		res.Classification = Internal
		return res, nil
	case c.lists.Disposable(domain):
		res.Classification = Disposable
		return res, nil
	case isFreeProvider(domain):
		res.Classification = Personal
		return res, nil
	}
	if class, ok := c.cache.get(domain); ok {
		res.Classification, res.Source = class, SourceCache
		return res, nil
	}
	if c.missing(ctx, domain) {
		res.Classification = Disposable
		return res, nil
	}
	if c.client == nil {
		return Result{}, ErrNotConfigured
	}
	res, err = c.ask(ctx, domain)
	if err != nil {
		return Result{}, err
	}
	if res.Classification != Unknown {
		c.cache.set(domain, res.Classification)
	}
	return res, nil
}

// missing reports whether DNS says the domain has neither a mail server nor
// an address. Any other DNS failure (a timeout, a refused or blocked query)
// is false, so the model decides and DNS never blocks a classification. The
// trailing dot skips the cluster's search domains.
func (c *Classifier) missing(ctx context.Context, domain string) bool {
	ctx, cancel := context.WithTimeout(ctx, c.mxTimeout)
	defer cancel()
	fqdn := domain + "."
	mx, err := c.resolver.LookupMX(ctx, fqdn)
	switch {
	case err == nil && len(mx) > 0:
		return false
	case err != nil && !notFound(err):
		slog.WarnContext(ctx, "email domain classify: mail server lookup failed; asking the model", "err", dnsReason(err))
		return false
	}
	addrs, err := c.resolver.LookupHost(ctx, fqdn)
	switch {
	case err == nil:
		return len(addrs) == 0
	case notFound(err):
		return true
	default:
		slog.WarnContext(ctx, "email domain classify: address lookup failed; asking the model", "err", dnsReason(err))
		return false
	}
}

// dnsReason is the DNS error without the name looked up.
func dnsReason(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Err
	}
	return "lookup failed"
}

func notFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

const systemPrompt = `You classify the email domain a person used to sign up for a B2B developer product.
Choose exactly one category:
- corporate: the domain of a real company, organisation, university or government body.
- personal: a consumer email provider people use for personal mail.
- disposable: a temporary, throwaway, anonymous or "burner" mail service, even one on no blocklist.
- unknown: you cannot tell.

Use your own knowledge of the domain. The domain is not on our small lists of free and disposable providers; that rules nothing out. Answer only with the structured field.`

var answerSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"category": map[string]any{"type": "string", "enum": []string{"corporate", "personal", "disposable", "unknown"}},
	},
	"required":             []string{"category"},
	"additionalProperties": false,
}

// ask makes one model call about domain.
func (c *Classifier) ask(ctx context.Context, domain string) (Result, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	start := time.Now()
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:        anthropic.Model(c.model),
		MaxTokens:    maxTokens,
		System:       []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:     []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("Domain: " + domain))},
		OutputConfig: anthropic.OutputConfigParam{Format: anthropic.JSONOutputFormatParam{Schema: answerSchema}},
	})
	took := time.Since(start)
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
			logUsage(ctx, c.model, "usage_limit", "", anthropic.Usage{}, took)
			return Result{}, fmt.Errorf("%w: %w: %w", ErrCall, ErrUsageLimit, err)
		}
		logUsage(ctx, c.model, "failed", "", anthropic.Usage{}, took)
		return Result{}, fmt.Errorf("%w: %w", ErrCall, err)
	}
	res, outcome, err := readAnswer(msg)
	logUsage(ctx, c.model, outcome, msg.StopReason, msg.Usage, took)
	res.Domain = domain
	return res, err
}

func readAnswer(msg *anthropic.Message) (Result, string, error) {
	switch msg.StopReason {
	case anthropic.StopReasonRefusal:
		return Result{}, "refused", ErrRefused
	case anthropic.StopReasonMaxTokens:
		return Result{}, "truncated", ErrTruncated
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	var answer struct {
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(text.String()), &answer); err != nil {
		return Result{}, "unusable", fmt.Errorf("%w: not the answer's JSON", ErrUnusable)
	}
	class := Classification(strings.ToUpper(answer.Category))
	switch class {
	case Corporate, Personal, Disposable, Unknown:
	default:
		return Result{}, "unusable", fmt.Errorf("%w: unexpected category", ErrUnusable)
	}
	return Result{Classification: class, Source: SourceModel,
		InputTokens: msg.Usage.InputTokens, OutputTokens: msg.Usage.OutputTokens}, "ok", nil
}

// logUsage writes the token usage line for one model call. Never the domain.
func logUsage(ctx context.Context, model, outcome string, stopReason anthropic.StopReason, usage anthropic.Usage, took time.Duration) {
	slog.InfoContext(ctx, "email domain classify: token usage",
		"model", model, "outcome", outcome, "stopReason", stopReason,
		"inputTokens", usage.InputTokens, "outputTokens", usage.OutputTokens,
		"durationMs", took.Milliseconds())
}
