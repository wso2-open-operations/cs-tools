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
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestParseIncidentFieldFiltersPostgres_RejectsUnappliedField proves a filter
// the shared contract accepts but this data source cannot apply is a 400, not
// silently dropped (which would widen the result set).
func TestParseIncidentFieldFiltersPostgres_RejectsUnappliedField(t *testing.T) {
	f := domain.SearchIncidentsFilters{Filters: []domain.IncidentFieldFilter{
		{Field: "incidentStateKeys", Op: "in", Values: []string{"1", "2"}},
	}}
	_, _, _, _, _, _, _, _, err := parseIncidentFieldFiltersPostgres(f, time.Now())
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != "filters: field incidentStateKeys is not supported by this data source" {
		t.Fatalf("err = %v, want a ValidationError naming incidentStateKeys", err)
	}
}
