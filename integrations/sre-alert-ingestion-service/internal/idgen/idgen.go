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

// Package idgen generates a buffered alert row's own primary-key UUID
// client-side, in internal/handler, before the row is ever persisted.
//
// Previously alert_buffer.id was left entirely to Postgres's
// gen_random_uuid() column default (see migrations/0001_create_alert_buffer.up.sql),
// generated only once internal/store.PostgresStore.Enqueue's INSERT
// returned. That ordering doesn't work for the dedup tag internal/handler
// now embeds in CreateIncidentRequest.Subject (see internal/csmclient.DedupTag):
// the tag must already be inside the JSON payload that gets persisted, which
// means the id it's derived from has to exist *before* the INSERT, not
// after. Hence generating it here instead.
package idgen

import (
	"crypto/rand"
	"fmt"
)

// New returns a random UUID v4 string (RFC 4122 textual form: e.g.
// "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed"), the same shape Postgres's
// gen_random_uuid() would have produced — alert_buffer.id is still a UUID
// PRIMARY KEY column, this service just supplies the value instead of
// relying on the column default now.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("idgen: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
