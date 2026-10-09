/*
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"context"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
)

// plgAudit records the outcome of a PLG write on this side of the hop.
//
// WHY THE CORRELATION ID IS PASSED EXPLICITLY. This service installs no slog
// handler — there is no slog.SetDefault anywhere in it — so a slog.*Context call
// carries only the attributes handed to it, and the id sitting in the context is
// silently dropped. The request logger has one solely because log.Printf
// interpolates it by hand. Reading it here is what lets a line in this service
// be matched to the BFF line that caused it; without it these records would be
// unjoinable to anything.
//
// WHY BOTH CALLER AND ACTOR. callerId is the identity the token carries, read
// from the validated x-user-id-token; actorId is the `"user".id` the BFF already
// resolved and the value this service writes into the *_by columns. They are
// different id spaces for the same person. A reader with only the first cannot
// find the row; with only the second cannot match the request log.
//
// actorID is "" for the writes whose service signature does not take one yet.
// Those are the four playbook writes, the organisation patch and the detach —
// the same set a separate change is threading an actor through. The field is
// omitted rather than logged empty, so a reader is not told the caller was
// unknown when the truth is that this operation does not carry one yet.
//
// NO PII. Ids and enum values only: never an organisation name, a person's
// name, an email, or note/task text.
func plgAudit(ctx context.Context, op, actorID string, err error, attrs ...any) {
	base := make([]any, 0, len(attrs)+8)
	base = append(base, "correlationID", middleware.CorrelationIDFromContext(ctx))
	if id := auth.IdentityFromContext(ctx); id.UserID != "" {
		base = append(base, "callerId", id.UserID)
	}
	if actorID != "" {
		base = append(base, "actorId", actorID)
	}
	base = append(base, attrs...)

	if err != nil {
		slog.ErrorContext(ctx, "plg "+op+": failed", append(base, "err", err)...)
		return
	}
	slog.InfoContext(ctx, "plg "+op+": success", base...)
}
