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
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestFoldRemainder(t *testing.T) {
	rows := []domain.CountByLabel{{Label: "a", Count: 5}, {Label: "b", Count: 4}, {Label: "c", Count: 2}, {Label: "d", Count: 1}}

	if got := foldRemainder(rows, 4); !reflect.DeepEqual(got, rows) {
		t.Errorf("at the cap nothing folds, got %+v", got)
	}
	got := foldRemainder(rows, 2)
	want := []domain.CountByLabel{{Label: "a", Count: 5}, {Label: "b", Count: 4}, {Label: otherLabel, Count: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("over the cap: got %+v, want %+v", got, want)
	}
	if rows[2].Label != "c" || len(rows) != 4 {
		t.Errorf("the input was changed: %+v", rows)
	}
}

// A visitor can name a source OTHER; it must not share a label with the folded remainder.
func TestFoldRemainderKeepsARealOtherSourceApart(t *testing.T) {
	rows := []domain.CountByLabel{{Label: "a", Count: 5}, {Label: "OTHER", Count: 4}, {Label: "c", Count: 2}, {Label: "d", Count: 1}}
	got := foldRemainder(rows, 2)
	want := []domain.CountByLabel{{Label: "a", Count: 5}, {Label: realOtherLabel, Count: 4}, {Label: otherLabel, Count: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	total := 0
	for _, r := range got {
		total += r.Count
	}
	if total != 12 {
		t.Errorf("total = %d, want 12", total)
	}
	if rows[1].Label != "OTHER" {
		t.Errorf("the input was changed: %+v", rows)
	}
}

func TestFoldQualificationRemainderAddsOutcomesAndKeepsTheTotal(t *testing.T) {
	q := func(label string, e, m, x, n int) domain.UTMQualification {
		return domain.UTMQualification{Label: label, Eligible: e, Monitor: m, Excluded: x, NotAssessed: n, Total: e + m + x + n}
	}
	rows := []domain.UTMQualification{q("a", 3, 2, 0, 1), q("other", 1, 1, 0, 0), q("c", 1, 2, 1, 0), q("d", 0, 1, 0, 2)}

	if got := foldQualificationRemainder(rows, 4); !reflect.DeepEqual(got, rows) {
		t.Errorf("at the cap nothing folds, got %+v", got)
	}
	got := foldQualificationRemainder(rows, 2)
	want := []domain.UTMQualification{q("a", 3, 2, 0, 1), q(realOtherLabel, 1, 1, 0, 0), q(otherLabel, 1, 3, 1, 2)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	total := 0
	for _, r := range got {
		total += r.Total
	}
	if total != 6+2+4+3 {
		t.Errorf("total = %d, want the whole cohort (%d)", total, 6+2+4+3)
	}
}
