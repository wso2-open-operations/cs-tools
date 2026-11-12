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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// csmOnlyCatalogItems is the part of CatalogRepository CreateCase needs to
// refuse a service request raised from a staff-only catalog item.
type csmOnlyCatalogItems interface {
	IsCatalogItemCSMOnly(ctx context.Context, catalogItemID string) (bool, error)
}

// WithCSMOnlyCatalogItemCheck attaches the lookup validateCSMOnlyCatalogItem
// uses, the same post-construction wiring as WithServiceRequestCatalog. A
// no-op if svc is not a *caseService or items is nil.
func WithCSMOnlyCatalogItemCheck(svc CaseService, items csmOnlyCatalogItems) CaseService {
	if cs, ok := svc.(*caseService); ok && items != nil {
		cs.csmOnlyItems = items
	}
	return svc
}

// validateCSMOnlyCatalogItem refuses a service request raised from a catalog
// item marked csm_only (migration 0291) unless the caller is WSO2 staff in the
// CSM portal (isCSMPortalStaff). The catalog already hides such items from
// everyone else; this is the check that holds when a caller posts the item id
// directly. A caller that can't be resolved is refused.
func (s *caseService) validateCSMOnlyCatalogItem(ctx context.Context, req domain.CreateCaseRequest) error {
	if s.csmOnlyItems == nil || req.Type != "service_request" || req.CatalogItemID == "" {
		return nil
	}
	csmOnly, err := s.csmOnlyItems.IsCatalogItemCSMOnly(ctx, req.CatalogItemID)
	if err != nil {
		return err
	}
	if !csmOnly || isCSMPortalStaff(ctx, s.access) {
		return nil
	}
	return &apierror.ForbiddenError{Msg: "this service request can only be raised by WSO2 staff from the CSM portal"}
}
