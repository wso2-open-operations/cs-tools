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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/viewer"
)

// entitySLAReport is entity-service's GET /projects/{id}/slas/report response.
// The summary rows and case rows have the same shape as the viewer types, so
// they are decoded straight into them.
type entitySLAReport struct {
	Project struct {
		Key  string  `json:"key"`
		Name *string `json:"name"`
	} `json:"project"`
	From    string                       `json:"from"`
	To      string                       `json:"to"`
	Summary []viewer.SLAReportSummaryRow `json:"summary"`
	Cases   []viewer.SLAReportCase       `json:"cases"`
}

// GetSLAReport builds the project SLA report. The project may be given as its
// entity-service id or its key; it is resolved to the id the same way the CS
// and time reports do. entity-service reads the clocks (sla_live) and does the
// percentile, so this only resolves the project and reshapes the response.
func (c *postgresReportsClient) GetSLAReport(ctx context.Context, projectID, from, to string) (viewer.SLAReportDetails, error) {
	project, err := c.resolveProject(ctx, projectID)
	if err != nil {
		return viewer.SLAReportDetails{}, err
	}
	raw, err := c.entity.GetProjectSLAReport(ctx, project.ID, from, to)
	if err != nil {
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return viewer.SLAReportDetails{}, viewer.ErrProjectNotFound
		}
		return viewer.SLAReportDetails{}, err
	}
	var report entitySLAReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return viewer.SLAReportDetails{}, fmt.Errorf("unmarshal entity-service SLA report: %w", err)
	}
	name := project.Name
	if report.Project.Name != nil {
		name = *report.Project.Name
	}
	details := viewer.SLAReportDetails{
		ProjectName: name,
		ProjectKey:  report.Project.Key,
		PeriodStart: report.From,
		PeriodEnd:   report.To,
		Summary:     report.Summary,
		Cases:       report.Cases,
	}
	if details.Summary == nil {
		details.Summary = []viewer.SLAReportSummaryRow{}
	}
	if details.Cases == nil {
		details.Cases = []viewer.SLAReportCase{}
	}
	return details, nil
}
