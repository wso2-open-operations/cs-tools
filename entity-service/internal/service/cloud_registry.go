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
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// cloudRegistryTTL bounds how stale the cloud registry may be.
//
// The registry is read-mostly configuration, so each service keeps a snapshot
// rather than reading three tables per request. PostgreSQL stays the source of
// truth: a cloud added, changed or retired in the database is picked up within
// this window, with no restart and no explicit invalidation to forget.
const cloudRegistryTTL = 30 * time.Second

// cloudRegistry serves the active-cloud snapshot, refreshing it at most once
// per TTL. It is safe for concurrent use.
type cloudRegistry struct {
	load func(ctx context.Context) ([]domain.CloudDefinition, error)
	ttl  time.Duration
	now  func() time.Time

	mu      sync.Mutex
	catalog domain.CloudCatalog
	loaded  bool
	at      time.Time
}

func newCloudRegistry(load func(ctx context.Context) ([]domain.CloudDefinition, error)) *cloudRegistry {
	return &cloudRegistry{load: load, ttl: cloudRegistryTTL, now: time.Now}
}

// Catalog returns the current snapshot. When a refresh fails it keeps serving
// the previous snapshot rather than turning a database blip into a dashboard
// outage; with no snapshot yet, the error is returned.
func (r *cloudRegistry) Catalog(ctx context.Context) (domain.CloudCatalog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.loaded && r.now().Sub(r.at) < r.ttl {
		return r.catalog, nil
	}
	defs, err := r.load(ctx)
	if err != nil {
		if r.loaded {
			slog.WarnContext(ctx, "cloud registry refresh failed; serving the previous snapshot", "err", err)
			return r.catalog, nil
		}
		return domain.CloudCatalog{}, err
	}
	r.catalog, r.loaded, r.at = domain.NewCloudCatalog(defs), true, r.now()
	return r.catalog, nil
}
