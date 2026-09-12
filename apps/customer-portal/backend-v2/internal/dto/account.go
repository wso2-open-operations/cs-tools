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

package dto

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

// AccountSummary is one item of the portal's response for POST /accounts/search.
//
// Normalizes entity-service's two data-source-dependent shapes (Postgres
// Account vs ServiceNow SNAccountView) into one: Tier prefers entity-service's
// "tier" and falls back to "classification"; SupportTier carries the plain
// label. Deliberately excludes entity-service's SfID (Salesforce internal
// ID), OwnerID/TechnicalOwnerID (bare internal IDs with no name — Owner/
// TechnicalOwner below is used instead when available), Pod (WSO2-internal
// account routing), CreatedBy (internal user ID), and ArrToday — annual
// recurring revenue is WSO2-internal financial data and must never reach the
// customer portal frontend.
type AccountSummary struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Tier             *string `json:"tier,omitempty"`
	Region           *string `json:"region,omitempty"`
	SupportTier      *string `json:"supportTier,omitempty"`
	Owner            *Ref    `json:"owner,omitempty"`
	TechnicalOwner   *Ref    `json:"technicalOwner,omitempty"`
	ActivationDate   *string `json:"activationDate,omitempty"`
	DeactivationDate *string `json:"deactivationDate,omitempty"`
	HasAgent         bool    `json:"hasAgent"`
	HasKbReferences  bool    `json:"hasKbReferences"`
	CreatedOn        *string `json:"createdOn,omitempty"`
	UpdatedOn        *string `json:"updatedOn,omitempty"`
}

// SearchAccountsResponse is the portal's response for POST /accounts/search.
type SearchAccountsResponse struct {
	Accounts     []AccountSummary `json:"accounts"`
	TotalRecords int              `json:"totalRecords"`
	Limit        int              `json:"limit"`
	Offset       int              `json:"offset"`
	HasMore      bool             `json:"hasMore"`
}

// MapSearchAccounts builds the portal response from entity-service's SearchAccountsResponse.
func MapSearchAccounts(r entity.SearchAccountsResponse) SearchAccountsResponse {
	accounts := make([]AccountSummary, 0, len(r.Accounts))
	for _, a := range r.Accounts {
		accounts = append(accounts, AccountSummary{
			ID:               a.ID,
			Name:             a.Name,
			Tier:             firstNonNil(a.Tier, a.Classification),
			Region:           a.Region,
			SupportTier:      a.SupportTier,
			Owner:            accountOwnerRef(a.Owner, a.AccountManager),
			TechnicalOwner:   mapAccountRef(a.TechnicalOwner),
			ActivationDate:   a.ActivationDate,
			DeactivationDate: a.DeactivationDate,
			HasAgent:         boolValue(firstNonNilBool(a.HasAgent, a.AgentEnabled)),
			HasKbReferences:  boolValue(firstNonNilBool(a.HasKbReferences, a.KbReferencesEnabled)),
			CreatedOn:        a.CreatedOn,
			UpdatedOn:        a.UpdatedOn,
		})
	}
	return SearchAccountsResponse{
		Accounts:     accounts,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}

// AccountDetails is the portal's response for GET /accounts/{id}. See
// AccountSummary's doc comment for the fields deliberately excluded here too.
type AccountDetails struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Tier             *string `json:"tier,omitempty"`
	Region           *string `json:"region,omitempty"`
	SupportTier      *string `json:"supportTier,omitempty"`
	Owner            *Ref    `json:"owner,omitempty"`
	TechnicalOwner   *Ref    `json:"technicalOwner,omitempty"`
	ActivationDate   *string `json:"activationDate,omitempty"`
	DeactivationDate *string `json:"deactivationDate,omitempty"`
	HasAgent         bool    `json:"hasAgent"`
	HasKbReferences  bool    `json:"hasKbReferences"`
	CreatedOn        *string `json:"createdOn,omitempty"`
	UpdatedOn        *string `json:"updatedOn,omitempty"`
}

// MapAccountDetails builds the portal response from entity-service's AccountDetail.
func MapAccountDetails(a entity.AccountDetail) AccountDetails {
	var supportTier *string
	if a.SupportTier != nil {
		supportTier = &a.SupportTier.Label
	}
	return AccountDetails{
		ID:               a.ID,
		Name:             a.Name,
		Tier:             firstNonNil(a.Tier, a.Classification),
		Region:           a.Region,
		SupportTier:      supportTier,
		Owner:            accountOwnerRef(a.Owner, a.AccountManager),
		TechnicalOwner:   mapAccountRef(a.TechnicalOwner),
		ActivationDate:   a.ActivationDate,
		DeactivationDate: a.DeactivationDate,
		HasAgent:         boolValue(firstNonNilBool(a.HasAgent, a.AgentEnabled)),
		HasKbReferences:  boolValue(firstNonNilBool(a.HasKbReferences, a.KbReferencesEnabled)),
		CreatedOn:        a.CreatedOn,
		UpdatedOn:        a.UpdatedOn,
	}
}

// mapAccountRef surfaces the ServiceNow {id,name} reference only. A bare
// Postgres owner/technicalOwner ID (no name attached) is intentionally
// dropped rather than exposed as a nameless Ref — see AccountSummary's doc
// comment.
// accountOwnerRef resolves the account's owning person for the portal's `owner`
// field. entity-service's unified account view renamed this `owner` ->
// `accountManager`; the old key is preferred when present so a deployment still
// emitting it is unaffected, and the new key is used otherwise. Without the
// fallback, `owner` serialises as null for every account under both data
// sources -- silently, since the field is optional.
func accountOwnerRef(owner, accountManager *entity.EntityRef) *Ref {
	if owner != nil {
		return mapAccountRef(owner)
	}
	return mapAccountRef(accountManager)
}

func mapAccountRef(ref *entity.EntityRef) *Ref {
	if ref == nil {
		return nil
	}
	return &Ref{ID: ref.ID, Name: ref.Name}
}

func firstNonNil(a, b *string) *string {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilBool(a, b *bool) *bool {
	if a != nil {
		return a
	}
	return b
}

func boolValue(b *bool) bool {
	return b != nil && *b
}
