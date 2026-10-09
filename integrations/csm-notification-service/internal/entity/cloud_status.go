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

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// ErrWebhookNotClaimable means entity-service refused the claim (409): the
// webhook is already delivered, or its reservation ran out and the scheduled
// task has it. The caller must not post.
var ErrWebhookNotClaimable = errors.New("entity: cloud status webhook is not claimable")

// ClaimCloudStatusWebhook starts the attempt to post one status-page webhook
// under the claim token outage.status_page_due carried. Post only on nil.
func (c *CustomerEntityClient) ClaimCloudStatusWebhook(ctx context.Context, webhookID, claimToken string) error {
	body, err := json.Marshal(struct {
		ClaimToken string `json:"claimToken"`
	}{claimToken})
	if err != nil {
		return fmt.Errorf("entity: encode cloud status claim: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, "/internal/cloud-status/"+url.PathEscape(webhookID)+"/claim", body)
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
		return ErrWebhookNotClaimable
	}
	return err
}

// CloudStatusDelivery is the outcome of one claimed attempt.
type CloudStatusDelivery struct {
	Delivered bool
	// Unknown: the request was sent and no answer came back; the dashboard
	// may have it, so it is never posted again automatically.
	Unknown bool
	Error   string
}

// RecordCloudStatusDelivery reports the outcome of a claimed attempt -- the
// call csm-scheduled-tasks makes after its own posts -- fenced by the claim
// token, so a stale report can never overwrite another outcome.
func (c *CustomerEntityClient) RecordCloudStatusDelivery(ctx context.Context, webhookID, claimToken string, d CloudStatusDelivery) error {
	if webhookID == "" {
		return fmt.Errorf("entity: webhookId is required to record a cloud status delivery")
	}
	body, err := json.Marshal(struct {
		Delivered  bool   `json:"delivered"`
		Error      string `json:"error,omitempty"`
		Unknown    bool   `json:"unknown,omitempty"`
		ClaimToken string `json:"claimToken,omitempty"`
	}{d.Delivered, d.Error, d.Unknown, claimToken})
	if err != nil {
		return fmt.Errorf("entity: encode cloud status delivery: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, "/internal/cloud-status/"+url.PathEscape(webhookID)+"/delivery", body)
	return err
}
