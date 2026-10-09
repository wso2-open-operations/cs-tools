// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package csm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type searchITServicesRequest struct {
	Filters    searchITServicesFilters `json:"filters,omitempty"`
	Pagination pagination              `json:"pagination"`
}

type searchITServicesFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

// ITService is the subset of entity-service's own ITService this client reads out of a search hit.
type ITService struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// SupportGroup is the CMDB group that supports the service; entity-service assigns an incident raised against it there.
	SupportGroup *ServiceGroup `json:"supportGroup,omitempty"`
}

// ServiceGroup is entity-service's EntityRef for a group.
type ServiceGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SupportGroupID is the support group's id, or "" when the service has none.
func (s ITService) SupportGroupID() string {
	if s.SupportGroup == nil {
		return ""
	}
	return s.SupportGroup.ID
}

type searchITServicesResponse struct {
	Services []ITService `json:"services"`
	Total    int         `json:"total"`
}

// Bounds SearchServiceID's walk; entity-service's search is substring-matched and ordered by created_on, not relevance.
const (
	servicesSearchPageSize = 50
	servicesSearchMaxPages = 20
)

// SearchService returns the CMDB service whose Name matches label case-insensitively; zero matches returns found=false, not an error.
func (c *Client) SearchService(ctx context.Context, label string) (svc ITService, found bool, err error) {
	offset := 0
	for page := 0; page < servicesSearchMaxPages; page++ {
		req := searchITServicesRequest{
			Filters:    searchITServicesFilters{SearchQuery: label},
			Pagination: pagination{Limit: servicesSearchPageSize, Offset: offset},
		}
		body, err := json.Marshal(req)
		if err != nil {
			return ITService{}, false, fmt.Errorf("csm: marshal SearchITServicesRequest: %w", err)
		}

		respBody, err := c.do(ctx, http.MethodPost, "/services/search", body)
		if err != nil {
			return ITService{}, false, err
		}

		var resp searchITServicesResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return ITService{}, false, fmt.Errorf("csm: decode SearchITServices response: %w", err)
		}

		if svc, ok := matchService(resp.Services, label); ok {
			return svc, true, nil
		}

		offset += len(resp.Services)
		if offset >= resp.Total {
			return ITService{}, false, nil
		}
	}
	return ITService{}, false, nil
}

// matchService picks the hit whose Name equals label ignoring case; the search itself is a substring match.
func matchService(services []ITService, label string) (ITService, bool) {
	for _, svc := range services {
		if svc.ID != "" && strings.EqualFold(svc.Name, label) {
			return svc, true
		}
	}
	return ITService{}, false
}
