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
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/upstreamhttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// SalesEntityConfig holds the configuration for the SupportPortalLite
// sales-side entity GraphQL service client — a separate, differently
// authenticated GraphQL service from this repo's own
// CustomerEntityClient/EngineeringEntityClient upstreams (ported from the
// Ballerina modules/entity/sales.bal module and its salesClient in
// clients.bal).
type SalesEntityConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
}

// SalesEntityClient is a GraphQL client for the SupportPortalLite
// sales-side entity service, authenticated via the OAuth2 client
// credentials grant.
type SalesEntityClient struct {
	http    *http.Client
	baseURL string
}

// NewSalesEntityClient constructs a SalesEntityClient that authenticates
// against the sales-side entity GraphQL service using the OAuth2 client
// credentials grant type.
func NewSalesEntityClient(cfg SalesEntityConfig) *SalesEntityClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		upstreamhttp.TokenClient(tokenFetchTimeout))
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &SalesEntityClient{http: httpClient, baseURL: cfg.BaseURL}
}

// Contact is the sales-side entity service's representation of a contact,
// resolved by email — mirrors Ballerina modules/types.Contact.
type Contact struct {
	ID      string `json:"id"`
	Account struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Classification string `json:"classification"`
	} `json:"account"`
	Memberships []ContactMembership `json:"memberships"`
}

// ContactMembership is one entry of Contact.Memberships — mirrors Ballerina
// modules/types.Memberships.
type ContactMembership struct {
	SubscriptionID string `json:"subscriptionId"`
	State          string `json:"state"`
	Type           string `json:"type"`
}

type contactsData struct {
	Contacts []Contact `json:"contacts"`
}

// GetContactByEmail resolves a sales-side Contact by email. Returns (nil,
// nil) when no contact matches — mirrors the Ballerina
// getContactByEmail's "contacts.length() > 0 ? contacts[0] : ()" fallback,
// which is not an error case.
func (c *SalesEntityClient) GetContactByEmail(ctx context.Context, email string) (*Contact, error) {
	const query = `
		query contactByEmail($filter: ContactFilter) {
			contacts(filter: $filter) {
				id
				account {
					id
					name
					classification
				}
				memberships{
					subscriptionId
					state
					type
				}
			}
		}`

	data, err := doGraphQL[contactsData](ctx, c.http, c.baseURL, query,
		map[string]any{"filter": map[string]any{"email": email}}, "sales entity: getContactByEmail")
	if err != nil {
		return nil, err
	}
	if len(data.Contacts) == 0 {
		return nil, nil
	}
	return &data.Contacts[0], nil
}

// Subscription is the sales-side entity service's representation of a
// subscription, resolved by subscription key — mirrors Ballerina
// modules/types.Subscription. Members are requested in the GraphQL query
// (matching the Ballerina query verbatim) but are not decoded here: the
// Ballerina Subscription record does not declare that field either, and no
// caller of getSubscriptionByKey ever reads it (confirmed against
// service.bal's scan-user resource, the only caller).
type Subscription struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Key        string `json:"key"`
	CustomerID string `json:"customerId"`
}

type subscriptionsData struct {
	Subscriptions []Subscription `json:"subscriptions"`
}

// GetSubscriptionByKey resolves a Subscription by its subscription key.
// Returns (nil, nil) when no subscription matches.
func (c *SalesEntityClient) GetSubscriptionByKey(ctx context.Context, subscriptionKey string) (*Subscription, error) {
	const query = `
	query subscriptionBySubscriptionKey($filter: SubscriptionFilter!) {
		subscriptions(filter: $filter) {
			id
			name
				key
				customerId
					members {
				id
				subscriptionId
				contact{
					id
				}
				status
				}
		}
	}`

	data, err := doGraphQL[subscriptionsData](ctx, c.http, c.baseURL, query,
		map[string]any{"filter": map[string]any{"key": subscriptionKey}}, "sales entity: getSubscriptionByKey")
	if err != nil {
		return nil, err
	}
	if len(data.Subscriptions) == 0 {
		return nil, nil
	}
	return &data.Subscriptions[0], nil
}
