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
	"testing"
	"time"
)

func TestDecodeActiveAlertMuteRules(t *testing.T) {
	raw := `{"alertMuteRules":[{"id":"b1","number":"AMR0000001","name":"DB upgrade","description":"","timeZone":"Asia/Colombo",
		"startOn":"2026-11-06T10:00:00Z","endOn":"2026-11-06T12:00:00Z",
		"conditions":[{"field":"service","value":"svc"},{"field":"environment","value":"prod"}],
		"createdBy":"a@wso2.com","createdOn":"2026-11-06T09:00:00Z","endedBy":null,"endedOn":null}]}`
	rules, err := decodeActiveAlertMuteRules([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("rules = %+v, want one", rules)
	}
	b := rules[0]
	if b.ID != "b1" || b.Number != "AMR0000001" || len(b.Conditions) != 2 || b.Conditions[1].Field != "environment" ||
		!b.EndOn.Equal(time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("rule = %+v", b)
	}
	if rules, err := decodeActiveAlertMuteRules([]byte(`{"alertMuteRules":[]}`)); err != nil || len(rules) != 0 {
		t.Fatalf("empty = %v, %v", rules, err)
	}
	if _, err := decodeActiveAlertMuteRules([]byte(`not json`)); err == nil {
		t.Fatal("want a decode error")
	}
}
