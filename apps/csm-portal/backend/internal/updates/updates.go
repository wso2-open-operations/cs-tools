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

package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GetRecommendedUpdateLevels fetches recommended update levels for the given user
// and returns them mapped to the portal camelCase response shape.
func (c *Client) GetRecommendedUpdateLevels(ctx context.Context, userEmail string) ([]RecommendedUpdateLevel, error) {
	raw, err := c.do(ctx, http.MethodGet, "/updates/recommended-update-levels", nil, map[string]string{
		"user": userEmail,
	})
	if err != nil {
		return nil, err
	}

	var upstream []upstreamRecommendedUpdateLevel
	if err := json.Unmarshal(raw, &upstream); err != nil {
		return nil, fmt.Errorf("updates: decode recommended update levels: %w", err)
	}

	return mapRecommendedUpdateLevels(upstream), nil
}

// GetProductUpdateLevels fetches all product update levels and returns them
// mapped to the portal camelCase response shape.
func (c *Client) GetProductUpdateLevels(ctx context.Context) ([]ProductUpdateLevel, error) {
	raw, err := c.do(ctx, http.MethodGet, "/updates/product-update-levels", nil, nil)
	if err != nil {
		return nil, err
	}

	var upstream []upstreamProductUpdateLevel
	if err := json.Unmarshal(raw, &upstream); err != nil {
		return nil, fmt.Errorf("updates: decode product update levels: %w", err)
	}

	return mapProductUpdateLevels(upstream), nil
}

// SearchUpdatesBetweenUpdateLevels searches for update descriptions between the
// given update levels for the given user and returns them grouped by update level.
func (c *Client) SearchUpdatesBetweenUpdateLevels(ctx context.Context, payload SearchPayload, userEmail string) (map[string]UpdateLevelGroup, error) {
	reqBody, err := json.Marshal(buildSearchRequest(payload, userEmail))
	if err != nil {
		return nil, fmt.Errorf("updates: encode search request: %w", err)
	}

	raw, err := c.do(ctx, http.MethodPost, "/updates/descriptions", reqBody, nil)
	if err != nil {
		return nil, err
	}

	var upstream []upstreamUpdateDescription
	if err := json.Unmarshal(raw, &upstream); err != nil {
		return nil, fmt.Errorf("updates: decode update descriptions: %w", err)
	}

	return groupByUpdateLevel(upstream), nil
}
