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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// errDashboardAccount marks a generated account filter naming no account, or
// several: the handler's 422 with the reason (generate_account).
var errDashboardAccount = errors.New("unusable account filter")

// errDashboardAccountLookup marks an accounts search that failed while the
// names were being checked: the handler's 502.
var errDashboardAccountLookup = errors.New("account lookup failed")

// errMsgGenerateAccountLookup is errDashboardAccountLookup's message.
const errMsgGenerateAccountLookup = "The account names in the dashboard couldn't be checked. Try again."

// accountSearcher is the entity service's POST /accounts/search
// (entity.CustomerEntityClient.SearchAccounts): an account's name, Salesforce
// id or number containing the search query, ignoring case, among the live
// accounts the caller can see.
type accountSearcher interface {
	SearchAccounts(ctx context.Context, body []byte) ([]byte, error)
}

// searchedAccount is one account of an accounts search.
type searchedAccount struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Name   string `json:"name"`
}

// WithDashboardAccounts wraps a generator so that each "account" filter (a
// customer's name or account number, as the user wrote it) becomes an
// "accountId" filter with the same op ("in", or "notIn" to leave the
// accounts out) on the id of the one account each value names
// (namedResolver.resolveValue: an exact number wins, then an exact name); a
// value naming no account, or several, is an errDashboardAccount.
func WithDashboardAccounts(inner DashboardGenerator, accounts accountSearcher) DashboardGenerator {
	return &namedResolvingGenerator{inner: inner, kind: namedKind{
		field: "account", idField: "accountId", noun: "account", plural: "accounts",
		unusable: errDashboardAccount, lookup: errDashboardAccountLookup,
		search: func(ctx context.Context, written string) ([]namedRecord, bool, error) {
			body, err := json.Marshal(map[string]any{
				"filters":    map[string]string{"searchQuery": written},
				"pagination": map[string]int{"limit": namedSearchLimit, "offset": 0},
			})
			if err != nil {
				return nil, false, err
			}
			raw, err := accounts.SearchAccounts(ctx, body)
			if err != nil {
				return nil, false, err
			}
			var res struct {
				Accounts []searchedAccount `json:"accounts"`
				HasMore  bool              `json:"hasMore"`
			}
			if err := json.Unmarshal(raw, &res); err != nil {
				return nil, false, fmt.Errorf("read the accounts search: %w", err)
			}
			records := make([]namedRecord, len(res.Accounts))
			for i, a := range res.Accounts {
				records[i] = namedRecord{ID: a.ID, Code: a.Number, Name: a.Name}
			}
			return records, res.HasMore, nil
		},
	}}
}
