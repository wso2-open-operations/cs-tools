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

package eventbus

import (
	"context"
	"encoding/json"
	"time"
)

// ParkFunc is called for a record that has failed every attempt on the last
// tier that will ever retry it (see Run and processRecord): it should put
// the record somewhere durable an operator can find — in this service, a
// dedicated parking topic (see ParkedRecord) — so the failure is never
// lost silently. A non-nil return means parking itself failed; Run logs
// that at ERROR and commits the offset regardless (there is nothing
// further to fall back to), so the record is then gone for good outside
// the source topic's own retention window.
type ParkFunc func(ctx context.Context, record Record, handleErr error) error

// ParkedRecord is the value published to the parking topic for a record
// this service has given up retrying. It wraps the original record rather
// than replacing it: Record is the source record's value, byte for byte
// (as raw JSON when it was valid JSON, otherwise base64 in RecordBase64),
// and the published Kafka key is the source record's own key, so the
// record can be replayed by republishing Record to SourceTopic under that
// key with nothing re-derived — see the README's "Parked records" section
// for the procedure. Failure is a summary of the last error, with any
// upstream response body already removed (apierror.Summary); it is
// diagnostic, never something a replay needs.
type ParkedRecord struct {
	ParkedAt        time.Time       `json:"parkedAt"`
	Consumer        string          `json:"consumer"`
	SourceTopic     string          `json:"sourceTopic"`
	SourcePartition int             `json:"sourcePartition"`
	SourceOffset    int64           `json:"sourceOffset"`
	Key             string          `json:"key"`
	Attempts        int             `json:"attempts"`
	Failure         string          `json:"failure"`
	Record          json.RawMessage `json:"record,omitempty"`
	RecordBase64    []byte          `json:"recordBase64,omitempty"`
}

// NewParkedRecord builds the parking envelope for record. failure should
// already be redacted (see ParkedRecord.Failure).
func NewParkedRecord(consumer string, record Record, failure string) ParkedRecord {
	p := ParkedRecord{
		ParkedAt:        time.Now().UTC(),
		Consumer:        consumer,
		SourceTopic:     record.Topic,
		SourcePartition: record.Partition,
		SourceOffset:    record.Offset,
		Key:             string(record.Key),
		Attempts:        record.Attempt,
		Failure:         failure,
	}
	if json.Valid(record.Value) {
		p.Record = json.RawMessage(record.Value)
	} else {
		p.RecordBase64 = record.Value
	}
	return p
}

// Payload returns p as the original record's bytes again: Record when the
// value was JSON, RecordBase64 otherwise — what a replay republishes.
func (p ParkedRecord) Payload() []byte {
	if len(p.Record) > 0 {
		return []byte(p.Record)
	}
	return p.RecordBase64
}
