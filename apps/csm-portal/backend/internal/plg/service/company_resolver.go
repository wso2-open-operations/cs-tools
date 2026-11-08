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

package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/cloudclient"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

const (
	// companyLookupConcurrency bounds parallel lookups (two calls each; 600/min limit).
	companyLookupConcurrency = 8
	// maxResolveIDs matches entity-service's bound.
	maxResolveIDs = 50
)

// IdentitySource reads what entity-service stores to identify organisations.
type IdentitySource interface {
	Identities(ctx context.Context, ids []string) ([]domain.OrganizationIdentity, error)
}

// CloudLookup is the Cloud Directory API, as the resolver uses it.
type CloudLookup interface {
	GetOrg(ctx context.Context, orgID string) (*cloudclient.Org, error)
	GetUserEmail(ctx context.Context, userID string) (*cloudclient.UserEmail, error)
}

// CompanyResolver answers the reveal: unknown ids are left out, a stored name
// answers from the database, otherwise the Cloud Directory API is asked (404 is
// UNRESOLVED, any other failure FAILED). Nothing is cached, stored or logged.
type CompanyResolver struct {
	entity IdentitySource
	cloud  CloudLookup // nil when the Cloud API is not configured
}

// NewCompanyResolver builds a resolver; a nil cloud reports cloud organisations FAILED.
func NewCompanyResolver(entity IdentitySource, cloud CloudLookup) *CompanyResolver {
	return &CompanyResolver{entity: entity, cloud: cloud}
}

// Resolve answers the reveal for each organisation id.
func (r *CompanyResolver) Resolve(ctx context.Context, req domain.ResolveCompaniesRequest) (domain.ResolveCompaniesResponse, error) {
	switch {
	case len(req.OrganizationIDs) == 0:
		return domain.ResolveCompaniesResponse{}, &apierror.ValidationError{Msg: "organizationIds must not be empty"}
	case len(req.OrganizationIDs) > maxResolveIDs:
		return domain.ResolveCompaniesResponse{}, &apierror.ValidationError{Msg: "organizationIds cannot list more than 50 organisations"}
	}
	for _, id := range req.OrganizationIDs {
		if err := validateUUID("organizationIds", id); err != nil {
			return domain.ResolveCompaniesResponse{}, err
		}
	}

	stored, err := r.entity.Identities(ctx, req.OrganizationIDs)
	if err != nil {
		return domain.ResolveCompaniesResponse{}, err
	}

	out := make([]domain.CompanyIdentity, len(stored))
	owners := make([]string, len(stored))
	var cloudAt []int
	for i, s := range stored {
		out[i] = domain.CompanyIdentity{OrganizationID: s.OrganizationID, CompanyID: s.MoesifCompanyID}
		switch {
		case s.OrganizationName != nil:
			out[i].OrganizationName, out[i].Email = s.OrganizationName, s.RegisteredEmail
			out[i].Source = domain.IdentityFromDatabase
		case trimmed(s.MoesifCompanyID) == "":
			out[i].Source = domain.IdentityUnresolved
		default:
			owners[i] = trimmed(s.OwnerUserID)
			cloudAt = append(cloudAt, i)
		}
	}

	if len(cloudAt) > 0 {
		r.resolveFromCloud(ctx, out, owners, cloudAt)
	}
	return domain.ResolveCompaniesResponse{Identities: out}, nil
}

// resolveFromCloud fills out[i] for each i in at, companyLookupConcurrency at a time.
func (r *CompanyResolver) resolveFromCloud(ctx context.Context, out []domain.CompanyIdentity, owners []string, at []int) {
	if r.cloud == nil {
		slog.WarnContext(ctx, "plg reveal: cloud api is not configured", "companies", len(at))
		for _, i := range at {
			out[i].Source = domain.IdentityFailed
		}
		return
	}

	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, companyLookupConcurrency)
	)
	for _, i := range at {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			// Each goroutine writes only its own index.
			out[i].OrganizationName, out[i].Email, out[i].Source =
				r.lookup(ctx, trimmed(out[i].CompanyID), owners[i])
		}(i)
	}
	wg.Wait()
}

// lookup asks the cloud about one company and its owner; logs carry ids only.
func (r *CompanyResolver) lookup(ctx context.Context, companyID, ownerUserID string) (name, email *string, source domain.IdentitySource) {
	org, err := r.cloud.GetOrg(ctx, companyID)
	switch {
	case errors.Is(err, cloudclient.ErrNotFound):
		return nil, nil, domain.IdentityUnresolved
	case err != nil:
		slog.ErrorContext(ctx, "plg reveal: cloud org lookup failed", "companyId", companyID, "err", err)
		return nil, nil, domain.IdentityFailed
	}
	name = nonEmpty(org.FriendlyName)

	if ownerUserID == "" {
		// Older registrations have no owner.
		return name, nil, domain.IdentityFromCloud
	}
	user, err := r.cloud.GetUserEmail(ctx, ownerUserID)
	switch {
	case errors.Is(err, cloudclient.ErrNotFound):
		return name, nil, domain.IdentityFromCloud
	case err != nil:
		slog.ErrorContext(ctx, "plg reveal: cloud user email lookup failed", "companyId", companyID, "err", err)
		return name, nil, domain.IdentityFromCloud
	}
	// An owner from another organisation gets no email.
	if user.OrgUUID != "" && !strings.EqualFold(user.OrgUUID, companyID) {
		slog.ErrorContext(ctx, "plg reveal: owner belongs to a different org", "companyId", companyID)
		return name, nil, domain.IdentityFromCloud
	}
	return name, nonEmpty(user.Email), domain.IdentityFromCloud
}

func trimmed(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func nonEmpty(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}
