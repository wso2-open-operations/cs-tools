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
	"regexp"
	"strconv"
	"strings"
)

// CaseDetails is the portal-shaped representation of a ServiceNow
// sn_customerservice_case record — mirrors Ballerina
// modules/operations/types.bal's CaseDetails.
//
// casePriorityMap and caseStateFromSNMap (used below) and the
// notFoundError/ErrProjectNotFound helpers are defined in reports.go, in
// this same package — reused here rather than redefined, to avoid the
// duplicate-symbol collisions this migration's parallel-file-ownership
// convention is meant to prevent.
type CaseDetails struct {
	Number                  string `json:"number"`
	SysID                   string `json:"sysId"`
	CaseID                  string `json:"caseId"`
	Description             string `json:"description,omitempty"`
	ShortDescription        string `json:"shortDescription"`
	AssignedTo              string `json:"assignedTo,omitempty"`
	CaseType                string `json:"caseType"`
	Priority                string `json:"priority"`
	State                   string `json:"state"`
	OpenedBy                string `json:"openedBy"`
	OpenedAt                string `json:"openedAt"`
	AccountNumber           string `json:"accountNumber"`
	AccountName             string `json:"accountName"`
	IsGenAiUsageAllowed     bool   `json:"isGenAiUsageAllowed,omitempty"`
	ProjectNumber           string `json:"projectNumber"`
	ProjectKey              string `json:"projectKey"`
	ProductName             string `json:"productName"`
	LastWSO2CommentTime     string `json:"lastWSO2CommentTime,omitempty"`
	LastCustomerCommentTime string `json:"lastCustomerCommentTime,omitempty"`
	ProjectDeploymentName   string `json:"projectDeploymentName,omitempty"`
	ProjectDeploymentType   string `json:"projectDeploymentType,omitempty"`
}

// CaseDetailsWithCount is GetCases' response shape — mirrors Ballerina
// CaseDetailsWithCount.
type CaseDetailsWithCount struct {
	Count int           `json:"count"`
	Cases []CaseDetails `json:"cases"`
}

// snCaseList is the raw ServiceNow sn_customerservice_case row shape used
// by the endpoints in this file (case search/get/by-project) — a superset
// of reports.go's snCaseDetailsList, which only carries the narrower field
// set GetTimeLogBreakdown needs. Field names mirror Ballerina SNCaseDetails
// exactly.
type snCaseList struct {
	Result []struct {
		Number                  string `json:"number"`
		SysID                   string `json:"sys_id"`
		WSO2CaseID              string `json:"u_wso2_case_id"`
		Description             string `json:"description"`
		ShortDescription        string `json:"short_description"`
		AssignedToEmail         string `json:"assigned_to.email"`
		CaseTypeName            string `json:"u_case_type.u_name"`
		Priority                string `json:"priority"`
		State                   string `json:"state"`
		OpenedByName            string `json:"opened_by.name"`
		OpenedAt                string `json:"opened_at"`
		AccountNumber           string `json:"account.number"`
		AccountName             string `json:"account.name"`
		AccountGenAiResponse    string `json:"account.u_ai_gen_response"`
		ProjectNumber           string `json:"project.number"`
		ProjectKey              string `json:"project.u_project_key"`
		ProductName             string `json:"product.name"`
		LastWSO2CommentTime     string `json:"u_last_wso2_comment_time"`
		LastCustomerCommentTime string `json:"u_last_customer_comment_time"`
		ProjectDeploymentName   string `json:"u_project_deployment.u_name"`
		ProjectDeploymentType   string `json:"u_project_deployment.u_type"`
	} `json:"result"`
}

var codeBlockOpenRe = regexp.MustCompile(`\[code\]`)
var codeBlockCloseRe = regexp.MustCompile(`\[/code\]`)

// trimCodeBlock strips ServiceNow's "[code]"/"[/code]" wrapper markers —
// mirrors Ballerina operations.trimCodeBlock.
func trimCodeBlock(value string) string {
	value = codeBlockOpenRe.ReplaceAllString(value, "")
	return codeBlockCloseRe.ReplaceAllString(value, "")
}

func toCaseDetailsList(data snCaseList) []CaseDetails {
	results := make([]CaseDetails, 0, len(data.Result))
	for _, item := range data.Result {
		results = append(results, CaseDetails{
			Number:                  item.Number,
			SysID:                   item.SysID,
			CaseID:                  item.WSO2CaseID,
			Description:             trimCodeBlock(item.Description),
			ShortDescription:        item.ShortDescription,
			AssignedTo:              item.AssignedToEmail,
			CaseType:                item.CaseTypeName,
			Priority:                casePriorityMap[item.Priority],
			State:                   caseStateFromSNMap[item.State],
			OpenedBy:                item.OpenedByName,
			OpenedAt:                item.OpenedAt,
			AccountNumber:           item.AccountNumber,
			AccountName:             item.AccountName,
			IsGenAiUsageAllowed:     item.AccountGenAiResponse == "true",
			ProjectNumber:           item.ProjectNumber,
			ProjectKey:              item.ProjectKey,
			ProductName:             item.ProductName,
			LastWSO2CommentTime:     item.LastWSO2CommentTime,
			LastCustomerCommentTime: item.LastCustomerCommentTime,
			ProjectDeploymentName:   item.ProjectDeploymentName,
			ProjectDeploymentType:   item.ProjectDeploymentType,
		})
	}
	return results
}

// caseStateToSNMap mirrors Ballerina utils.caseStateToSNMap — the inverse
// of caseStateFromSNMap (reports.go), used when building a sysparm_query
// filter from a caller-supplied display label.
var caseStateToSNMap = map[string]string{
	"Open":              "1",
	"Work In Progress":  "10",
	"Awaiting Info":     "18",
	"Solution Proposed": "6",
	"Closed":            "3",
	"Cancelled":         "7",
	"In Progress":       "1001",
	"Waiting on Client": "1002",
	"Waiting on WSO2":   "1003",
	"Reopened":          "1006",
	"Differed":          "1007",
}

// GetCasesByProject retrieves the cases under the project with the given
// project ID, paginated, optionally filtered by case state and case type.
// Mirrors Ballerina operations:getCasesByProject. Caller-supplied
// stateFilters/caseTypeFilters values are looked up in caseStateToSNMap /
// used verbatim as ServiceNow case-type names respectively; caseTypeFilters
// entries are validated with SanitizeQueryValue since — unlike state
// filters, which are mapped through a fixed lookup table — they are
// concatenated into the query as-is.
func (c *Client) GetCasesByProject(ctx context.Context, projectID string, stateFilters, caseTypeFilters []string, offset, limit int) ([]CaseDetails, error) {
	if err := ValidateRecordNumberOrSysID(projectID); err != nil {
		return nil, err
	}
	query := "project.number=" + projectID + "^ORDERBYDESCsys_updated_on"

	if len(stateFilters) > 0 {
		var codes []string
		for _, sf := range stateFilters {
			if code, ok := caseStateToSNMap[sf]; ok {
				codes = append(codes, code)
			}
		}
		query += "^stateIN" + strings.Join(codes, ",") + ","
	}
	if len(caseTypeFilters) > 0 {
		for _, ct := range caseTypeFilters {
			if err := SanitizeQueryValue(ct); err != nil {
				return nil, err
			}
		}
		clauses := make([]string, 0, len(caseTypeFilters))
		clauses = append(clauses, "u_case_type.u_name="+caseTypeFilters[0])
		for _, ct := range caseTypeFilters[1:] {
			clauses = append(clauses, "u_case_type.u_name="+ct)
		}
		query += "^" + strings.Join(clauses, "^OR")
	}

	raw, err := c.TableQuery(ctx, "sn_customerservice_case", url.Values{
		"sysparm_query": {query},
		"sysparm_fields": {"number,sys_id,u_wso2_case_id,short_description," +
			"u_case_type.u_name,priority,state,opened_by.name,opened_at"},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	})
	if err != nil {
		return nil, err
	}
	var data snCaseList
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("servicenow: decode project cases response: %w", err)
	}
	return toCaseDetailsList(data), nil
}

// ErrCaseNotFound is returned when a case lookup by number finds no
// matching ServiceNow record.
var ErrCaseNotFound = errors.New("servicenow: no case found for that case number")

// GetCaseByNumber retrieves a single case by its case number. Returns
// ErrCaseNotFound when no case matches.
func (c *Client) GetCaseByNumber(ctx context.Context, caseNumber string) (CaseDetails, error) {
	if err := ValidateRecordNumber(caseNumber); err != nil {
		return CaseDetails{}, err
	}
	raw, err := c.TableQuery(ctx, "sn_customerservice_case", url.Values{
		"sysparm_query": {"number=" + caseNumber},
		"sysparm_fields": {"number,u_wso2_case_id,description,short_description,assigned_to.email," +
			"u_case_type.u_name,priority,state,opened_by.name,opened_at,account.number,account.name," +
			"account.u_ai_gen_response,project.number,project.u_project_key,product.name," +
			"u_last_wso2_comment_time,u_last_customer_comment_time," +
			"u_project_deployment.u_name,u_project_deployment.u_type"},
		"sysparm_limit": {"1"},
	})
	if err != nil {
		return CaseDetails{}, err
	}
	var data snCaseList
	if err := json.Unmarshal(raw, &data); err != nil {
		return CaseDetails{}, fmt.Errorf("servicenow: decode case response: %w", err)
	}
	if len(data.Result) == 0 {
		return CaseDetails{}, ErrCaseNotFound
	}
	return toCaseDetailsList(data)[0], nil
}

// getCaseSysID resolves caseNumber to its ServiceNow sys_id. Returns
// ErrCaseNotFound when no case matches.
func (c *Client) getCaseSysID(ctx context.Context, caseNumber string) (string, error) {
	raw, err := c.TableQuery(ctx, "sn_customerservice_case", url.Values{
		"sysparm_query":  {"number=" + caseNumber},
		"sysparm_fields": {"sys_id"},
		"sysparm_limit":  {"1"},
	})
	if err != nil {
		return "", err
	}
	var data snSysIDResultList
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("servicenow: decode case sys_id response: %w", err)
	}
	if len(data.Result) == 0 {
		return "", ErrCaseNotFound
	}
	return data.Result[0].SysID, nil
}

// Comment is a single case comment or work note — mirrors Ballerina
// modules/operations/types.bal's Comment.
type Comment struct {
	CreatedOn string `json:"createdOn"`
	Value     string `json:"value"`
	CreatedBy string `json:"createdBy"`
	Type      string `json:"type"`
}

// CommentsResponse is GetCommentsAndWorknotes' response shape — mirrors
// Ballerina CommentsResponse.
type CommentsResponse struct {
	Total    int       `json:"total"`
	Comments []Comment `json:"comments"`
}

type snSysJournal struct {
	SysCreatedOn string `json:"sys_created_on"`
	Value        string `json:"value"`
	SysCreatedBy string `json:"sys_created_by"`
	Element      string `json:"element"`
}

type snSysJournalList struct {
	Result []snSysJournal `json:"result"`
}

// totalCountHeader is the ServiceNow pagination header this backend reads —
// mirrors Ballerina constants.SERVICENOW_PAGINATION_TOTAL_COUNT_HEADER.
const totalCountHeader = "X-Total-Count"

// GetCommentsAndWorknotes retrieves the comments and work notes for the
// case with the given case number, paginated, with a total count. Mirrors
// Ballerina operations:getCommentsAndWorknotes. Returns ErrCaseNotFound
// when no case matches caseNumber.
func (c *Client) GetCommentsAndWorknotes(ctx context.Context, caseNumber string, offset, limit int) (CommentsResponse, error) {
	if err := ValidateRecordNumber(caseNumber); err != nil {
		return CommentsResponse{}, err
	}
	caseSysID, err := c.getCaseSysID(ctx, caseNumber)
	if err != nil {
		return CommentsResponse{}, err
	}

	// Get total count first, mirroring the Ballerina original's separate
	// sysparm_count=true call. "^" is ServiceNow's ORDERBY combinator; it's
	// passed here as a literal so url.Values.Encode() (used by
	// TableQueryWithHeaders) percent-encodes it to "%5E" on the wire, which
	// is what ServiceNow expects. A pre-encoded "%5E" would itself get
	// percent-encoded a second time (to "%255E"), and ServiceNow would
	// receive the literal text "%5E" instead of the "^" combinator.
	_, countHeaders, err := c.TableQueryWithHeaders(ctx, "sys_journal_field", url.Values{
		"sysparm_query":  {"element_id=" + caseSysID + "^ORDERBYDESCsys_created_on"},
		"sysparm_fields": {"sys_id"},
		"sysparm_count":  {"true"},
	})
	if err != nil {
		return CommentsResponse{}, err
	}
	total, err := strconv.Atoi(countHeaders.Get(totalCountHeader))
	if err != nil {
		return CommentsResponse{}, fmt.Errorf("servicenow: invalid %s header value %q: %w", totalCountHeader, countHeaders.Get(totalCountHeader), err)
	}

	raw, err := c.TableQuery(ctx, "sys_journal_field", url.Values{
		"sysparm_query":  {"element_id=" + caseSysID + "^ORDERBYDESCsys_created_on"},
		"sysparm_fields": {"sys_created_on,value,sys_created_by,element"},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	})
	if err != nil {
		return CommentsResponse{}, err
	}
	var journal snSysJournalList
	if err := json.Unmarshal(raw, &journal); err != nil {
		return CommentsResponse{}, fmt.Errorf("servicenow: decode comments response: %w", err)
	}

	comments := make([]Comment, 0, len(journal.Result))
	for _, item := range journal.Result {
		comments = append(comments, Comment{
			CreatedOn: item.SysCreatedOn,
			Value:     trimCodeBlock(item.Value),
			CreatedBy: item.SysCreatedBy,
			Type:      item.Element,
		})
	}
	return CommentsResponse{Total: total, Comments: comments}, nil
}

// AttachmentInfo is a case attachment's metadata — mirrors Ballerina
// modules/operations/types.bal's AttachmentInfo.
type AttachmentInfo struct {
	SysID       string `json:"sysId"`
	FileName    string `json:"fileName"`
	CreatedOn   string `json:"createdOn"`
	CreatedBy   string `json:"createdBy"`
	UpdatedOn   string `json:"updatedOn"`
	UpdatedBy   string `json:"updatedBy"`
	ContentType string `json:"contentType"`
	State       string `json:"state"`
}

type snAttachment struct {
	SysID       string `json:"sys_id"`
	FileName    string `json:"file_name"`
	CreatedOn   string `json:"sys_created_on"`
	CreatedBy   string `json:"sys_created_by"`
	UpdatedOn   string `json:"sys_updated_on"`
	UpdatedBy   string `json:"sys_updated_by"`
	ContentType string `json:"content_type"`
	State       string `json:"state"`
}

type snAttachmentList struct {
	Result []snAttachment `json:"result"`
}

// GetAttachmentsInfo retrieves the attachment metadata for the case with
// the given case number, paginated. Mirrors Ballerina
// operations:getAttachmentsInfo. Returns ErrCaseNotFound when no case
// matches caseNumber.
func (c *Client) GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]AttachmentInfo, error) {
	if err := ValidateRecordNumber(caseNumber); err != nil {
		return nil, err
	}
	caseSysID, err := c.getCaseSysID(ctx, caseNumber)
	if err != nil {
		return nil, err
	}

	raw, err := c.TableQuery(ctx, "sys_attachment", url.Values{
		"sysparm_query": {"table_sys_id=" + caseSysID},
		"sysparm_fields": {"sys_id,file_name,sys_created_on,sys_created_by," +
			"sys_updated_on,sys_updated_by,content_type,state"},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	})
	if err != nil {
		return nil, err
	}
	var data snAttachmentList
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("servicenow: decode attachments response: %w", err)
	}

	results := make([]AttachmentInfo, 0, len(data.Result))
	for _, item := range data.Result {
		results = append(results, AttachmentInfo(item))
	}
	return results, nil
}

// GetCases searches cases by free-text search string or by state filter
// (mutually exclusive in the Ballerina original — see
// operations:getCases — replicated as-is: stateFilter, when set, takes over
// the query entirely rather than combining with searchString).
func (c *Client) GetCases(ctx context.Context, searchString, stateFilter *string, offset, limit int) (CaseDetailsWithCount, error) {
	query := ""
	if searchString != nil {
		if err := SanitizeQueryValue(*searchString); err != nil {
			return CaseDetailsWithCount{}, err
		}
		query = "numberLIKE" + *searchString + "^ORu_wso2_case_idLIKE" + *searchString
	}
	if stateFilter != nil {
		stateCode := caseStateToSNMap[*stateFilter]
		query = "project.u_project_type.u_name=Subscription^ORproject.u_project_type.u_name=Managed Cloud Subscription" +
			"^state=" + stateCode + "^u_case_type.u_name!=Announcement" +
			"^u_case_type.u_name!=Hosting^u_case_type.u_name!=Hosting Query^u_case_type.u_name!=Hosting Task^u_case_type.u_name!=Sub-Task" +
			"^u_case_type.u_name!=Engagement^ORDERBYDESCsys_created_on"
	}

	raw, headers, err := c.TableQueryWithHeaders(ctx, "sn_customerservice_case", url.Values{
		"sysparm_query": {query},
		"sysparm_fields": {"number,u_wso2_case_id,short_description," +
			"u_case_type.u_name,priority,state,opened_by.name,opened_at"},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	})
	if err != nil {
		return CaseDetailsWithCount{}, err
	}
	count, err := strconv.Atoi(headers.Get(totalCountHeader))
	if err != nil {
		return CaseDetailsWithCount{}, fmt.Errorf("servicenow: invalid %s header value %q: %w", totalCountHeader, headers.Get(totalCountHeader), err)
	}

	var data snCaseList
	if err := json.Unmarshal(raw, &data); err != nil {
		return CaseDetailsWithCount{}, fmt.Errorf("servicenow: decode cases response: %w", err)
	}

	return CaseDetailsWithCount{Count: count, Cases: toCaseDetailsList(data)}, nil
}
