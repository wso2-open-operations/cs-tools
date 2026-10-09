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
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// outageCloudStatusTimeout bounds the status-page work after an outage write:
// a few queries and one publish. It runs on a context detached from the
// request, so a client hanging up cannot cut it off half way.
const outageCloudStatusTimeout = 15 * time.Second

// outageCloudStatus tells the public status page about an outage the moment
// it is declared or ended, instead of on csm-scheduled-tasks' next tick.
//
// THE WRITE IS THE EVENT. This service writes `outage` itself now (the
// portal's Begin / End outage), so the transition is known here, in the
// request that made it -- no trigger, no poller. After a successful create or
// update it runs the same decision the sweep runs (CloudStatusService
// HandleOutages: in scope? which clouds? begin or end?), which records each
// new transition once and publishes it as outage.status_page_due for
// csm-notification-service to post.
//
// Best effort, after the commit: the outage is written whatever happens here,
// and anything missed is still recorded and posted by the scheduled task.
type outageCloudStatus struct {
	OutageService
	cloudStatus CloudStatusService
}

// WithOutageCloudStatus wraps an outage service so its writes reach the
// status page at once. A nil cloudStatus returns svc unchanged.
func WithOutageCloudStatus(svc OutageService, cloudStatus CloudStatusService) OutageService {
	if cloudStatus == nil {
		return svc
	}
	return &outageCloudStatus{OutageService: svc, cloudStatus: cloudStatus}
}

// CreateOutage implements OutageService.
func (s *outageCloudStatus) CreateOutage(ctx context.Context, req domain.CreateOutageRequest) (domain.CreateOutageResponse, error) {
	resp, err := s.OutageService.CreateOutage(ctx, req)
	if err == nil {
		s.notify(ctx, resp.Outage.ID)
	}
	return resp, err
}

// UpdateOutage implements OutageService. Every successful update is handed
// over, not only one that sets the end: the decision is derived from the
// outage's state, so an update that changes nothing it cares about records
// and publishes nothing.
func (s *outageCloudStatus) UpdateOutage(ctx context.Context, req domain.PatchOutageRequest) (domain.PatchOutageResponse, error) {
	resp, err := s.OutageService.UpdateOutage(ctx, req)
	if err == nil {
		s.notify(ctx, req.ID)
	}
	return resp, err
}

func (s *outageCloudStatus) notify(ctx context.Context, outageID string) {
	if outageID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), outageCloudStatusTimeout)
	defer cancel()
	if err := s.cloudStatus.HandleOutages(ctx, []string{outageID}); err != nil {
		slog.ErrorContext(ctx, "cloudstatus: status page update after outage write failed; the scheduled sweep will catch it",
			"outageId", outageID, "err", err)
	}
}
