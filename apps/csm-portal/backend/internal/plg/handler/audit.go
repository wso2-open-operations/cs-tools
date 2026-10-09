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
	"log/slog"
	"net/http"
)

// auditWrite records the outcome of a PLG write.
//
// WHY EVERY WRITE AND NOT ONLY FAILURES. writeServiceError already logs when a
// handler returns an error, so before this a successful write left nothing but
// the request line: a method, a path and a 200. That says a DELETE happened; it
// does not say which run was detached, from which pairing, or by whom. An audit
// trail is a record of what was done, so the success path is the half that
// matters and the half that was missing.
//
// THE THREE IDENTITY FIELDS, and why all three are needed:
//
//   - correlationID — added automatically by the BFF's ctxHandler, which is why
//     it is absent from the attribute list below. It is what ties this line to
//     the same request's line in entity-service.
//   - userID — the Asgardeo "userid" claim, also added automatically by
//     ctxHandler, which is why it is absent below too. entity-service records
//     the same value under the name callerId; they are one identity, and a line
//     from each service can be matched on it.
//   - actorId — the resolved `"user".id`, which is what the database's *_by
//     columns actually store.
//
// The last two are DIFFERENT IDENTIFIERS FOR THE SAME PERSON, in different id
// spaces, and neither can be derived from the other without the user table.
// Recording both is what lets a log line be matched to a database row.
//
// NO PII. Ids and enum values only: never an organisation name, a person's name,
// an email, or note/task text. The caller is identified by id in both spaces,
// which is sufficient to answer "who" without naming anybody.
func auditWrite(r *http.Request, op string, err error, attrs ...any) {
	// Only actorId is added by hand. correlationID and userID are already on
	// every record from this process — adding callerId here as well produced two
	// names for one value on every line, which is noise, not redundancy.
	base := make([]any, 0, len(attrs)+4)
	base = append(base, "actorId", actor(r))
	base = append(base, attrs...)

	if err != nil {
		// The message carries the reason; writeServiceError logs the mapped
		// status separately, so this is the domain half of the same failure.
		slog.ErrorContext(r.Context(), "plg "+op+": failed", append(base, "err", err)...)
		return
	}
	slog.InfoContext(r.Context(), "plg "+op+": success", base...)
}

// enumPtr renders an optional enum for a log attribute.
//
// The pairing patch carries three independently-optional axes; a nil one means
// "not sent", which is different from a value and worth distinguishing in the
// record. Returning "" for nil keeps the key present with an empty value rather
// than dereferencing a nil pointer.
func enumPtr[T ~string](v *T) string {
	if v == nil {
		return ""
	}
	return string(*v)
}
