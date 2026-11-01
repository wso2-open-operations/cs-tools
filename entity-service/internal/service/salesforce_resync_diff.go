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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// resyncFieldSpec is one field of a resync comparison: the stored value, the
// value the ingest's mapper would write, and what is needed to tell a length
// cut from a plain difference. The comparison is pure so it can be tested
// without a database or Salesforce.
type resyncFieldSpec struct {
	name string
	// stored is the value in the database (nil for NULL, or when the row has
	// no value for it).
	stored *string
	// written is what the ingest would write: the mapper's output, after its
	// length guard. nil is a NULL write.
	written *string
	// raw is the Salesforce value (trimmed) before the length guard; nil when
	// the field has no guard or Salesforce sent nothing.
	raw *string
	// limit is the column width in characters the guard cuts to; 0 means the
	// column is not bounded.
	limit int
	// numeric compares stored and written as numbers ("5" equals "5.00").
	numeric bool
	// keepStoredWhenNil marks a column the ingest leaves alone when
	// Salesforce sends nothing (a COALESCE on write): a nil written value is
	// not a difference.
	keepStoredWhenNil bool
}

// diffResyncFields returns the fields whose stored value differs from the
// value the ingest would write. SfValue is the written value, or the full
// Salesforce value when the guard still cuts it (WouldTruncate): then the
// admin sees what Salesforce holds and that CSM will store less of it.
func diffResyncFields(specs []resyncFieldSpec) []domain.SalesforceResyncFieldDiff {
	diffs := []domain.SalesforceResyncFieldDiff{}
	for _, f := range specs {
		if f.keepStoredWhenNil && f.written == nil {
			continue
		}
		if resyncValuesEqual(f.stored, f.written, f.numeric) {
			continue
		}
		truncates := f.limit > 0 && f.raw != nil && utf8.RuneCountInString(*f.raw) > f.limit
		sf := f.written
		if truncates {
			sf = f.raw
		}
		diffs = append(diffs, domain.SalesforceResyncFieldDiff{
			Field:         f.name,
			CsmValue:      f.stored,
			SfValue:       sf,
			WouldTruncate: truncates,
		})
	}
	return diffs
}

func resyncValuesEqual(stored, written *string, numeric bool) bool {
	if stored == nil || written == nil {
		return stored == nil && written == nil
	}
	if *stored == *written {
		return true
	}
	if numeric {
		a, errA := strconv.ParseFloat(strings.TrimSpace(*stored), 64)
		b, errB := strconv.ParseFloat(strings.TrimSpace(*written), 64)
		return errA == nil && errB == nil && math.Abs(a-b) < 1e-9
	}
	return false
}

// resyncText is a trimmed text value, nil when empty (the ingest's
// optionalPtr on a plain string).
func resyncText(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// resyncDateText renders a mapped DATE as the YYYY-MM-DD text the stored
// columns are read as.
func resyncDateText(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

func resyncNumText(f *float64) *string {
	if f == nil {
		return nil
	}
	s := strconv.FormatFloat(*f, 'f', -1, 64)
	return &s
}

func resyncBoolText(b *bool) *string {
	if b == nil {
		return nil
	}
	s := strconv.FormatBool(*b)
	return &s
}

// resyncSfIDText is a Salesforce id in its 18-character form, nil when empty,
// so a 15-character id and its 18-character twin compare equal.
func resyncSfIDText(s string) *string {
	s = salesforceID18(strings.TrimSpace(s))
	if s == "" {
		return nil
	}
	return &s
}

// resyncStoredSfID is resyncSfIDText for a stored value.
func resyncStoredSfID(v *string) *string {
	if v == nil {
		return nil
	}
	return resyncSfIDText(*v)
}

// resyncComponentBuilder tallies one component's records.
type resyncComponentBuilder struct {
	comp domain.SalesforceResyncComponent
}

func newResyncComponent(key, label string) *resyncComponentBuilder {
	return &resyncComponentBuilder{comp: domain.SalesforceResyncComponent{
		Key: key, Label: label, Status: domain.SalesforceResyncStatusInSync,
		Records: []domain.SalesforceResyncRecord{},
	}}
}

func (b *resyncComponentBuilder) inSync() { b.comp.Counts.InSync++ }

func (b *resyncComponentBuilder) differs(sfID, label string, fields []domain.SalesforceResyncFieldDiff) {
	b.comp.Counts.Differs++
	b.comp.Records = append(b.comp.Records, domain.SalesforceResyncRecord{
		SfID: sfID, Label: label, State: domain.SalesforceResyncRecordDiffers, Fields: fields,
	})
}

func (b *resyncComponentBuilder) missingInCSM(sfID, label string) {
	b.comp.Counts.MissingInCSM++
	b.comp.Records = append(b.comp.Records, domain.SalesforceResyncRecord{
		SfID: sfID, Label: label, State: domain.SalesforceResyncRecordMissingInCSM,
		Fields: []domain.SalesforceResyncFieldDiff{},
	})
}

func (b *resyncComponentBuilder) onlyInCSM(sfID, label string) {
	b.comp.Counts.OnlyInCSM++
	b.comp.Records = append(b.comp.Records, domain.SalesforceResyncRecord{
		SfID: sfID, Label: label, State: domain.SalesforceResyncRecordOnlyInCSM,
		Fields: []domain.SalesforceResyncFieldDiff{},
	})
}

// compare records one Salesforce record against its stored fields: in sync
// when no field differs.
func (b *resyncComponentBuilder) compare(sfID, label string, fields []domain.SalesforceResyncFieldDiff) {
	if len(fields) == 0 {
		b.inSync()
		return
	}
	b.differs(sfID, label, fields)
}

// finish sets the status. A row only CSM holds is informational (the apply
// never removes it), so only a difference or a missing row makes a component
// "differs".
func (b *resyncComponentBuilder) finish() domain.SalesforceResyncComponent {
	if b.comp.Counts.Differs > 0 || b.comp.Counts.MissingInCSM > 0 {
		b.comp.Status = domain.SalesforceResyncStatusDiffers
	}
	return b.comp
}

// skippedResyncComponent is a component that was not compared, and why.
func skippedResyncComponent(key, label, reason string) domain.SalesforceResyncComponent {
	return domain.SalesforceResyncComponent{
		Key: key, Label: label, Status: domain.SalesforceResyncStatusSkipped,
		SkippedReason: &reason, Records: []domain.SalesforceResyncRecord{},
	}
}
