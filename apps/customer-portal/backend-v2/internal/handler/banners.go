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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// entityBannerClient is the customer-portal read of portal banners.
type entityBannerClient interface {
	ListCustomerBanners(ctx context.Context) (entity.PortalBannerViewList, error)
}

// BannerHandler serves GET /banners. The entity-service route always returns
// the customer audience.
type BannerHandler struct {
	entity entityBannerClient
}

// NewBannerHandler creates a BannerHandler.
func NewBannerHandler(entity entityBannerClient) *BannerHandler {
	return &BannerHandler{entity: entity}
}

// ListBanners handles GET /banners.
func (h *BannerHandler) ListBanners(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	list, err := h.entity.ListCustomerBanners(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCustomerBanners failed", "err", err)
		mapUpstreamError(w, err, "Failed to load banners.")
		return
	}
	writeJSONValue(w, http.StatusOK, dto.MapPortalBannerViewList(list))
}

// Compile-time check that the entity client satisfies the handler.
var _ entityBannerClient = (*entity.Client)(nil)
