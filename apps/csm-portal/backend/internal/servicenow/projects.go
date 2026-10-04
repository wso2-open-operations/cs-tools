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

package servicenow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

// ProjectDetails is the portal-shaped representation of a ServiceNow
// customer_project record — mirrors Ballerina
// modules/operations/types.bal's ProjectDetails.
type ProjectDetails struct {
	Number              string `json:"number"`
	SysID               string `json:"sysId"`
	Name                string `json:"name"`
	Key                 string `json:"key"`
	StartDate           string `json:"startDate"`
	EndDate             string `json:"endDate"`
	RemainingQueryHours string `json:"remainingQueryHours"`
	ClosureState        string `json:"closureState"`
	ProjectType         string `json:"projectType"`
	AccountNumber       string `json:"accountNumber"`
	AccountName         string `json:"accountName"`
	TotalQueryHours     string `json:"totalQueryHours"`
}

// snProjectDetails is the raw ServiceNow customer_project row shape —
// mirrors Ballerina SNProjectDetails.
type snProjectDetails struct {
	Number              string `json:"number"`
	SysID               string `json:"sys_id"`
	ShortDescription    string `json:"short_description"`
	ProjectKey          string `json:"u_project_key"`
	Start               string `json:"start"`
	End                 string `json:"end"`
	RemainingQueryHours string `json:"u_remaining_query_hours"`
	ClosureState        string `json:"u_wso2_closure_state"`
	ProjectTypeName     string `json:"u_project_type.u_name"`
	AccountNumber       string `json:"account.number"`
	AccountName         string `json:"account.name"`
	TotalQueryHours     string `json:"u_total_query_hour"`
}

type snProjectData struct {
	Result []snProjectDetails `json:"result"`
}

func toProjectDetailsList(data snProjectData) []ProjectDetails {
	results := make([]ProjectDetails, 0, len(data.Result))
	for _, item := range data.Result {
		results = append(results, ProjectDetails{
			Number:              item.Number,
			SysID:               item.SysID,
			Name:                item.ShortDescription,
			Key:                 item.ProjectKey,
			StartDate:           item.Start,
			EndDate:             item.End,
			RemainingQueryHours: item.RemainingQueryHours,
			ClosureState:        item.ClosureState,
			ProjectType:         item.ProjectTypeName,
			AccountNumber:       item.AccountNumber,
			AccountName:         item.AccountName,
			TotalQueryHours:     item.TotalQueryHours,
		})
	}
	return results
}

// GetProjectsByAccount retrieves the projects under the account with the
// given account number, paginated. Mirrors Ballerina
// operations:getProjectsByAccount.
func (c *Client) GetProjectsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]ProjectDetails, error) {
	if err := ValidateRecordNumberOrSysID(accountNumber); err != nil {
		return nil, err
	}
	raw, err := c.TableQuery(ctx, "customer_project", url.Values{
		"sysparm_query":  {"account.number=" + accountNumber},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	})
	if err != nil {
		return nil, err
	}
	var data snProjectData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("servicenow: decode account projects response: %w", err)
	}
	return toProjectDetailsList(data), nil
}

// GetProjects retrieves projects with pagination, optionally filtered by a
// short-description search phrase. Mirrors Ballerina
// operations:getProjects.
func (c *Client) GetProjects(ctx context.Context, phrase *string, offset, limit int) ([]ProjectDetails, error) {
	query := ""
	if phrase != nil {
		if err := SanitizeQueryValue(*phrase); err != nil {
			return nil, err
		}
		query = "short_descriptionLIKE" + *phrase
	}

	raw, err := c.TableQuery(ctx, "customer_project", url.Values{
		"sysparm_query": {query},
		"sysparm_fields": {"number, short_description, u_project_key, start," +
			"end, u_remaining_query_hours, u_wso2_closure_state," +
			"u_project_type.u_name"},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	})
	if err != nil {
		return nil, err
	}
	var data snProjectData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("servicenow: decode projects response: %w", err)
	}
	return toProjectDetailsList(data), nil
}

// ErrProjectByIDNotFound is returned by GetProjectByID when no project
// matches the given project number.
var ErrProjectByIDNotFound = errors.New("servicenow: no project found for that project number")

// GetProjectByID retrieves a single project by its project number. Returns
// ErrProjectByIDNotFound when no project matches.
func (c *Client) GetProjectByID(ctx context.Context, projectID string) (ProjectDetails, error) {
	if err := ValidateRecordNumberOrSysID(projectID); err != nil {
		return ProjectDetails{}, err
	}
	raw, err := c.TableQuery(ctx, "customer_project", url.Values{
		"sysparm_query": {"number=" + projectID},
		"sysparm_fields": {"number,sys_id, short_description, u_project_key, start," +
			"end, u_remaining_query_hours, u_wso2_closure_state," +
			"u_project_type.u_name,account.number,account.name,u_total_query_hour"},
		"sysparm_limit": {"1"},
	})
	if err != nil {
		return ProjectDetails{}, err
	}
	var data snProjectData
	if err := json.Unmarshal(raw, &data); err != nil {
		return ProjectDetails{}, fmt.Errorf("servicenow: decode project response: %w", err)
	}
	if len(data.Result) == 0 {
		return ProjectDetails{}, ErrProjectByIDNotFound
	}
	return toProjectDetailsList(data)[0], nil
}

// Contact is a project's contact — mirrors Ballerina
// modules/operations/types.bal's Contact.
type Contact struct {
	ContactName string `json:"contactName"`
	Email       string `json:"email"`
	State       string `json:"state"`
}

type snContact struct {
	CustomerContactName string `json:"customer_contact.name"`
	Email               string `json:"u_email"`
	State               string `json:"u_state"`
}

type snContactList struct {
	Result []snContact `json:"result"`
}

// GetProjectContacts retrieves the contacts for the project with the given
// project ID, paginated. Mirrors Ballerina operations:getProjectContacts.
// The sysparm_query value uses a literal "=" — url.Values.Encode() (used by
// TableQuery) percent-encodes it to "%3D" on the wire, which is what
// ServiceNow expects. A pre-encoded "%3D" here would itself get
// percent-encoded a second time (to "%253D"), and ServiceNow would receive
// the literal text "%3D" instead of the "=" operator.
func (c *Client) GetProjectContacts(ctx context.Context, projectID string, offset, limit int) ([]Contact, error) {
	if err := ValidateRecordNumberOrSysID(projectID); err != nil {
		return nil, err
	}
	raw, err := c.TableQuery(ctx, "project_contact", url.Values{
		"sysparm_query":  {"customer_project.number=" + projectID},
		"sysparm_fields": {"customer_contact.name,u_email,u_state"},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	})
	if err != nil {
		return nil, err
	}
	var data snContactList
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("servicenow: decode project contacts response: %w", err)
	}

	results := make([]Contact, 0, len(data.Result))
	for _, item := range data.Result {
		state := item.State
		if state == "" {
			state = "-"
		}
		results = append(results, Contact{
			ContactName: item.CustomerContactName,
			Email:       item.Email,
			State:       state,
		})
	}
	return results, nil
}
