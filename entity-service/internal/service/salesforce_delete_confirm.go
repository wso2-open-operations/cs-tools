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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// confirmDeletedUpstream decides whether a DELETED event may be applied. The
// event envelope names only a record id, so it is never taken on its own: the
// record is looked up again first (fetchErr is that lookup's result) and the
// delete goes ahead only when the upstream answers that the record no longer
// exists.
//
//   - lookup succeeded: the record is still there, so the event is
//     acknowledged without changing anything (gone=false, err=nil). This also
//     covers a DELETED that arrives after a newer RESTORED.
//   - lookup reported the record missing: apply the delete (gone=true).
//   - any other failure: the answer is unknown, so return the error and let
//     the event be retried rather than deleting on a guess.
func confirmDeletedUpstream(ctx context.Context, entity, sfID string, fetchErr error) (gone bool, err error) {
	switch {
	case fetchErr == nil:
		slog.WarnContext(ctx, "salesforce: DELETED event ignored, the record is still present upstream", "entity", entity, "sfId", sfID)
		return false, nil
	case errors.Is(fetchErr, salesentity.ErrNotFound):
		return true, nil
	default:
		return false, fetchErr
	}
}

// errDeleteUnconfirmable is returned when a DELETED event arrives for an
// entity whose upstream lookup is not wired, so the delete cannot be
// confirmed and must not be applied.
var errDeleteUnconfirmable = errors.New("salesforce: DELETED event cannot be confirmed upstream, no lookup client configured")
