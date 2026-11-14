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

package dispatch

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
)

// StaffLookup says whether the person behind an email address is an internal
// user (WSO2 staff, such as a support engineer), by the user type
// entity-service holds. The address's domain says nothing reliable: a few
// customers use one, and a person's type is what the portals go by.
type StaffLookup interface {
	IsInternal(ctx context.Context, email string) bool
}

// userTypeSearcher is the slice of entity.CustomerEntityClient this needs.
type userTypeSearcher interface {
	SearchUsersByEmail(ctx context.Context, emails []string) ([]entity.UserRoleInfo, error)
}

// staffLookupTimeout bounds one lookup: the icon is a courtesy, and an alert
// must never wait on it. When it runs out the answer is "not internal".
const staffLookupTimeout = 2 * time.Second

type staffEntry struct {
	internal bool
	expires  time.Time
}

type cachedStaffLookup struct {
	search userTypeSearcher
	ttl    time.Duration
	now    func() time.Time

	mu    sync.Mutex
	cache map[string]staffEntry
}

// NewStaffLookup answers from entity-service's user search, remembering each
// answer for ttl so a busy case does not search again for every email. A
// failed lookup answers "not internal" and is not remembered: the icon is a
// courtesy, and an alert must never wait on or fail because of it.
func NewStaffLookup(search userTypeSearcher, ttl time.Duration) StaffLookup {
	return &cachedStaffLookup{search: search, ttl: ttl, now: time.Now, cache: map[string]staffEntry{}}
}

func (l *cachedStaffLookup) IsInternal(ctx context.Context, email string) bool {
	key := strings.ToLower(strings.TrimSpace(email))
	if key == "" || l.search == nil {
		return false
	}
	l.mu.Lock()
	if e, ok := l.cache[key]; ok && l.now().Before(e.expires) {
		l.mu.Unlock()
		return e.internal
	}
	l.mu.Unlock()

	lookupCtx, cancel := context.WithTimeout(ctx, staffLookupTimeout)
	defer cancel()
	users, err := l.search.SearchUsersByEmail(lookupCtx, []string{key})
	if err != nil {
		slog.WarnContext(ctx, "dispatch: could not look up a user's type for the support engineer icon", "err", err)
		return false
	}
	internal := false
	for _, u := range users {
		if strings.EqualFold(strings.TrimSpace(u.Email), key) && strings.EqualFold(u.UserType, "internal") {
			internal = true
			break
		}
	}
	l.mu.Lock()
	l.cache[key] = staffEntry{internal: internal, expires: l.now().Add(l.ttl)}
	l.mu.Unlock()
	return internal
}
