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

package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// GoogleChatSpace maps a single product to the Google Chat space that should
// receive its incident alerts.
type GoogleChatSpace struct {
	// Product identifies the product this space is dedicated to (e.g.
	// "api-manager", "identity-server"). Matched case-insensitively against
	// the product passed to SendIncidentAlert.
	Product string `json:"product"`
	// WebhookURL is that space's incoming webhook URL (Space settings > Apps
	// & integrations > Webhooks). It already carries its own key/token query
	// parameters, so no separate auth flow is needed.
	WebhookURL string `json:"webhookUrl"`
}

// GoogleChatConfig holds the configuration for the Google Chat notification
// channel: one space per product, since each WSO2 product has its own space.
type GoogleChatConfig struct {
	Spaces []GoogleChatSpace
}

// GoogleChatClient posts messages to a Google Chat space via an incoming
// webhook, routing each alert to the space configured for the case's
// product. Unlike the OAuth2-authenticated clients in this package, a
// webhook URL is the only credential required.
//
// NewGoogleChatClient never fails, so it is safe to construct with a
// zero-value GoogleChatConfig (e.g. when this channel is not yet configured
// for a given deployment) — a missing or unmatched product only surfaces as
// an error the first time SendIncidentAlert is called for it.
type GoogleChatClient struct {
	http                 *http.Client
	webhookURLsByProduct map[string]string
}

// NewGoogleChatClient constructs a GoogleChatClient that routes alerts to the
// webhook configured for each product in cfg.Spaces.
func NewGoogleChatClient(cfg GoogleChatConfig) *GoogleChatClient {
	webhookURLsByProduct := make(map[string]string, len(cfg.Spaces))
	for _, space := range cfg.Spaces {
		product := normalizeProduct(space.Product)
		if product == "" || strings.TrimSpace(space.WebhookURL) == "" {
			continue
		}
		// A second space normalizing to the same product (e.g. "API-Manager"
		// and " api-manager ") is a configuration mistake — mark it
		// unconfigured rather than silently routing to whichever URL came
		// last.
		if _, exists := webhookURLsByProduct[product]; exists {
			webhookURLsByProduct[product] = ""
			continue
		}
		webhookURLsByProduct[product] = space.WebhookURL
	}
	return &GoogleChatClient{
		http:                 &http.Client{Timeout: 10 * time.Second},
		webhookURLsByProduct: webhookURLsByProduct,
	}
}

// normalizeProduct makes product matching case- and whitespace-insensitive.
func normalizeProduct(product string) string {
	return strings.ToLower(strings.TrimSpace(product))
}

// redactURLError strips the request URL — which carries the webhook's secret
// key/token query parameters — out of a *url.Error before it's wrapped and
// potentially logged, keeping only the underlying (safe) failure reason.
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

// chatCardMessage is the wire shape Google Chat's webhook API expects for a
// single card message: https://developers.google.com/chat/api/guides/message-formats/cards
type chatCardMessage struct {
	CardsV2 []chatCardWrapper `json:"cardsV2"`
}

type chatCardWrapper struct {
	CardID string   `json:"cardId"`
	Card   chatCard `json:"card"`
}

type chatCard struct {
	Header   chatCardHeader    `json:"header"`
	Sections []chatCardSection `json:"sections"`
}

type chatCardHeader struct {
	Title string `json:"title"`
}

type chatCardSection struct {
	Header  string           `json:"header,omitempty"`
	Widgets []chatCardWidget `json:"widgets"`
}

// chatCardWidget is a union type: exactly one of TextParagraph or ButtonList
// is set per widget, matching Google Chat's widget schema.
type chatCardWidget struct {
	TextParagraph *chatTextParagraph `json:"textParagraph,omitempty"`
	ButtonList    *chatButtonList    `json:"buttonList,omitempty"`
}

type chatTextParagraph struct {
	Text string `json:"text"`
}

type chatButtonList struct {
	Buttons []chatButton `json:"buttons"`
}

type chatButton struct {
	Text    string      `json:"text"`
	OnClick chatOnClick `json:"onClick"`
}

type chatOnClick struct {
	OpenLink chatOpenLink `json:"openLink"`
}

type chatOpenLink struct {
	URL string `json:"url"`
}

// SendIncidentAlert posts a card message announcing a newly created
// incident/case, with a button linking back to the case in the CSM portal,
// to the Google Chat space configured for the given product.
func (c *GoogleChatClient) SendIncidentAlert(ctx context.Context, product, title, shortDescription, portalURL string) error {
	if title == "" {
		return fmt.Errorf("notifications: title is required")
	}
	webhookURL, ok := c.webhookURLsByProduct[normalizeProduct(product)]
	if !ok || webhookURL == "" {
		return fmt.Errorf("notifications: no google chat space configured for product %q", product)
	}

	msg := chatCardMessage{
		CardsV2: []chatCardWrapper{
			{
				CardID: "incident-alert",
				Card: chatCard{
					Header: chatCardHeader{Title: title},
					Sections: []chatCardSection{
						{
							Header: "Short Description",
							Widgets: []chatCardWidget{
								{TextParagraph: &chatTextParagraph{Text: shortDescription}},
							},
						},
						{
							Widgets: []chatCardWidget{
								{
									ButtonList: &chatButtonList{
										Buttons: []chatButton{
											{
												Text:    "Open in CSM Portal",
												OnClick: chatOnClick{OpenLink: chatOpenLink{URL: portalURL}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("notifications: encode google chat message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notifications: build google chat request: %w", redactURLError(err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notifications: post google chat message: %w", redactURLError(err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("notifications: read google chat response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return nil
}
