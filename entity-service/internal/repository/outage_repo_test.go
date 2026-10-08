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

package repository

import (
	"strings"
	"testing"
)

// nativeOutageInsertSQL is the insert exactly as it was before the external
// id/number path existed. The native path must stay byte-identical to it.
const nativeOutageInsertSQL = `
INSERT INTO outage (id, number, type, start_on, end_on, name,
                    service_offering_id, work_item_id,
                    external_outage_communications, internal_outage_communications,
                    notify_internal_stakeholders, outage_communication, impact, state, duration,
                    created_on, created_by, updated_on, updated_by)
VALUES (gen_random_uuid(),
        'OUT' || LPAD(nextval('outage_number_seq')::text, 7, '0'),
        $1::outage_type_enum, $2, $3, $4,
        $5::uuid, $6::uuid, $7, $8, $10, $11, $12, $13,
        -- ServiceNow's "Outage Calculations" business rule: duration is
        -- end - begin, and NULL while either is missing. Readers such as the
        -- outage-communication email take it from this column, so an outage
        -- created here must carry it like a synced one does.
        $3::timestamptz - $2::timestamptz,
        NOW(), $9, NOW(), $9)
RETURNING id::text`

func TestOutageInsertStatement_NativeWhenNumberAndIDUnset(t *testing.T) {
	for name, in := range map[string]OutageWrite{
		"neither":     {},
		"number only": {Number: "OUT0001234"},
		"id only":     {ID: "11111111-2222-3333-4444-555555555555"},
	} {
		sql, extra := outageInsertStatement(in)
		if sql != nativeOutageInsertSQL {
			t.Errorf("%s: native SQL changed:\n%s", name, sql)
		}
		if len(extra) != 0 {
			t.Errorf("%s: native path must bind no extra args, got %v", name, extra)
		}
		if !strings.Contains(sql, "nextval('outage_number_seq')") || !strings.Contains(sql, "gen_random_uuid()") {
			t.Errorf("%s: native path lost its generators", name)
		}
	}
}

func TestOutageInsertStatement_UsesExternalNumberAndID(t *testing.T) {
	const id, number = "11111111-2222-3333-4444-555555555555", "OUT0001234"
	sql, extra := outageInsertStatement(OutageWrite{ID: id, Number: number})
	if strings.Contains(sql, "outage_number_seq") || strings.Contains(sql, "gen_random_uuid") {
		t.Errorf("external path must not use the native generators:\n%s", sql)
	}
	if !strings.Contains(sql, "VALUES ($14::uuid,\n        $15,") {
		t.Errorf("external path must bind id=$14 and number=$15:\n%s", sql)
	}
	if len(extra) != 2 || extra[0] != id || extra[1] != number {
		t.Errorf("extra args = %v, want [%s %s]", extra, id, number)
	}
}
