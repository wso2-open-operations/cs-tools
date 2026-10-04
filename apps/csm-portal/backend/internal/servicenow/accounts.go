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

// AccountDetails is the portal-shaped representation of a ServiceNow
// customer_account record — mirrors Ballerina
// modules/operations/types.bal's AccountDetails.
type AccountDetails struct {
	Number                        string `json:"number"`
	Name                          string `json:"name"`
	Region                        string `json:"region"`
	Country                       string `json:"country"`
	City                          string `json:"city"`
	ARR                           string `json:"arr"`
	AccountManager                string `json:"accountManager"`
	TechnicalOwner                string `json:"technicalOwner"`
	CustomerSuccessManager        string `json:"customerSuccessManager"`
	Rating                        string `json:"rating"`
	IntegrationCSTeamName         string `json:"integrationCSTeamName"`
	IntegrationCSTeamEmail        string `json:"integrationCSTeamEmail"`
	IntegrationCSTeamManagerName  string `json:"integrationCSTeamManagerName"`
	IntegrationCSTeamManagerEmail string `json:"integrationCSTeamManagerEmail"`
	IntegrationCSTeamSysID        string `json:"integrationCSTeamSysId"`
	IntegrationCSTeamScheduleURL  string `json:"integrationCSTeamScheduleUrl"`
	DriveLocation                 string `json:"driveLocation"`
}

// snAccountDetails is the raw ServiceNow customer_account row shape — field
// names mirror the exact sysparm_fields dot-path column names ServiceNow
// returns (e.g. "u_owner.name"), not Go-idiomatic names, matching Ballerina
// SNAccountDetails.
type snAccountDetails struct {
	Number                        string `json:"number"`
	Name                          string `json:"name"`
	ARRToday                      string `json:"u_arr_today"`
	Region                        string `json:"u_region"`
	Country                       string `json:"country"`
	City                          string `json:"city"`
	OwnerName                     string `json:"u_owner.name"`
	TechnicalOwnerName            string `json:"u_technical_owner.name"`
	CustomerSuccessManagerName    string `json:"u_customer_success_manager.name"`
	Rating                        string `json:"u_rating"`
	SysID                         string `json:"sys_id"`
	IntegrationCSTeamName         string `json:"u_integration_cs_team.name"`
	IntegrationCSTeamEmail        string `json:"u_integration_cs_team.email"`
	IntegrationCSTeamManagerName  string `json:"u_integration_cs_team.manager.name"`
	IntegrationCSTeamManagerEmail string `json:"u_integration_cs_team.manager.email"`
	IntegrationCSTeamSysID        string `json:"u_integration_cs_team.sys_id"`
	DriveLocation                 string `json:"u_drive_location"`
}

type snAccountData struct {
	Result []snAccountDetails `json:"result"`
}

// teamScheduleURL is prepended to a resolved integration-CS-team sys_id to
// build AccountDetails.IntegrationCSTeamScheduleURL — mirrors Ballerina's
// configurable teamScheduleUrl, used the same way in operations.bal's
// getAccountDetails. Set once at Client construction.
func (c *Client) toAccountDetailsList(data snAccountData) []AccountDetails {
	results := make([]AccountDetails, 0, len(data.Result))
	for _, item := range data.Result {
		results = append(results, AccountDetails{
			Number:                        item.Number,
			Name:                          item.Name,
			Region:                        item.Region,
			Country:                       item.Country,
			City:                          item.City,
			ARR:                           item.ARRToday,
			AccountManager:                item.OwnerName,
			TechnicalOwner:                item.TechnicalOwnerName,
			CustomerSuccessManager:        item.CustomerSuccessManagerName,
			Rating:                        item.Rating,
			IntegrationCSTeamName:         item.IntegrationCSTeamName,
			IntegrationCSTeamEmail:        item.IntegrationCSTeamEmail,
			IntegrationCSTeamManagerName:  item.IntegrationCSTeamManagerName,
			IntegrationCSTeamManagerEmail: item.IntegrationCSTeamManagerEmail,
			IntegrationCSTeamSysID:        item.IntegrationCSTeamSysID,
			IntegrationCSTeamScheduleURL:  c.teamScheduleURL + item.IntegrationCSTeamSysID,
			DriveLocation:                 item.DriveLocation,
		})
	}
	return results
}

// GetAccounts retrieves accounts with pagination, optionally filtered by
// owner email, ownership type ("TO"/"AM"), a name search phrase, and
// active-only. Mirrors Ballerina operations:getAccounts. email and phrase
// are validated with SanitizeQueryValue before being concatenated into the
// sysparm_query — return an *ErrUnsafeQueryValue to the caller as a 400.
func (c *Client) GetAccounts(ctx context.Context, email, userType, phrase *string, offset, limit int, active bool) ([]AccountDetails, error) {
	var clauses []string
	if email != nil {
		if err := SanitizeQueryValue(*email); err != nil {
			return nil, err
		}
		switch userType {
		case nil:
			clauses = append(clauses, fmt.Sprintf("u_owner.email=%s^ORu_renewal_account_manager.email=%s^ORu_technical_owner.email=%s", *email, *email, *email))
		default:
			switch *userType {
			case "TO":
				clauses = append(clauses, "u_technical_owner.email="+*email)
			case "AM":
				clauses = append(clauses, fmt.Sprintf("u_owner.email=%s^ORu_renewal_account_manager.email=%s", *email, *email))
			default:
				clauses = append(clauses, fmt.Sprintf("u_owner.email=%s^ORu_renewal_account_manager.email=%s^ORu_technical_owner.email=%s", *email, *email, *email))
			}
		}
		if phrase != nil {
			if err := SanitizeQueryValue(*phrase); err != nil {
				return nil, err
			}
			clauses[len(clauses)-1] += "^nameLIKE" + *phrase
		}
	} else if phrase != nil {
		if err := SanitizeQueryValue(*phrase); err != nil {
			return nil, err
		}
		clauses = append(clauses, "nameLIKE"+*phrase)
	}

	query := BuildEncodedQuery(clauses...)
	if active {
		activeClause := "u_nameNOT LIKEzzz^ORnameNOT LIKEzzz^u_statusNOT LIKElost"
		if query != "" {
			query += "^" + activeClause
		} else {
			query = activeClause
		}
	}

	params := url.Values{
		"sysparm_query":  {query},
		"sysparm_fields": {"name, number, u_arr_today, u_owner.name, u_technical_owner.name, u_customer_success_manager.name, u_region, country, city, u_drive_location, u_renewal_account_manager.name"},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	}

	raw, err := c.TableQuery(ctx, "customer_account", params)
	if err != nil {
		return nil, err
	}
	var data snAccountData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("servicenow: decode accounts response: %w", err)
	}
	return c.toAccountDetailsList(data), nil
}

// ErrAccountNotFound is returned by GetAccountByID when no account matches
// the given account number.
var ErrAccountNotFound = errors.New("servicenow: no account found for that account number")

// GetAccountByID retrieves a single account by its account number. Returns
// ErrAccountNotFound when no account matches.
func (c *Client) GetAccountByID(ctx context.Context, accountNumber string) (AccountDetails, error) {
	if err := ValidateRecordNumberOrSysID(accountNumber); err != nil {
		return AccountDetails{}, err
	}
	params := url.Values{
		"sysparm_query": {"number=" + accountNumber},
		"sysparm_fields": {"name, number, u_arr_today, u_owner.name, u_technical_owner.name," +
			"u_customer_success_manager.name, u_region, u_rating, country, city, u_integration_cs_team.name," +
			"u_integration_cs_team.email, u_integration_cs_team.manager.name,u_integration_cs_team.manager.email," +
			"u_integration_cs_team.sys_id, u_drive_location"},
		"sysparm_limit": {"1"},
	}

	raw, err := c.TableQuery(ctx, "customer_account", params)
	if err != nil {
		return AccountDetails{}, err
	}
	var data snAccountData
	if err := json.Unmarshal(raw, &data); err != nil {
		return AccountDetails{}, fmt.Errorf("servicenow: decode account response: %w", err)
	}
	if len(data.Result) == 0 {
		return AccountDetails{}, ErrAccountNotFound
	}
	return c.toAccountDetailsList(data)[0], nil
}

// EscalationDetail is the portal-shaped representation of an escalation —
// mirrors Ballerina EscalationDetail.
type EscalationDetail struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	State       string `json:"state"`
	EscalatedOn string `json:"escalatedOn"`
}

type snEscalationDetail struct {
	Number             string `json:"number"`
	State              string `json:"state"`
	EscalationSeverity string `json:"escalation_severity.name"`
	OpenedAt           string `json:"opened_at"`
}

type snEscalations struct {
	Result []snEscalationDetail `json:"result"`
}

// escalationStateLabels mirrors the match expression in Ballerina
// getEscalationDetails.
var escalationStateLabels = map[string]string{
	"100": "Requested",
	"101": "Escalated",
	"102": "Declined",
	"103": "Closed",
	"104": "De-escalation Requested",
}

// GetEscalationsByAccount retrieves the escalations for the account with
// the given account number, paginated. Returns ErrAccountNotFound when no
// account matches accountNumber.
func (c *Client) GetEscalationsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]EscalationDetail, error) {
	if err := ValidateRecordNumberOrSysID(accountNumber); err != nil {
		return nil, err
	}

	accountParams := url.Values{
		"sysparm_query":  {"number=" + accountNumber},
		"sysparm_fields": {"name, number, u_arr_today, u_owner.name, u_technical_owner.name, u_region, sys_id"},
		"sysparm_limit":  {"1"},
	}
	accountRaw, err := c.TableQuery(ctx, "customer_account", accountParams)
	if err != nil {
		return nil, err
	}
	var accountData snAccountData
	if err := json.Unmarshal(accountRaw, &accountData); err != nil {
		return nil, fmt.Errorf("servicenow: decode account response: %w", err)
	}
	if len(accountData.Result) == 0 {
		return nil, ErrAccountNotFound
	}
	sysID := accountData.Result[0].SysID

	escParams := url.Values{
		"sysparm_query":  {"source_record=" + sysID},
		"sysparm_fields": {"number, state, escalation_severity.name, opened_at"},
		"sysparm_limit":  {strconv.Itoa(limit)},
		"sysparm_offset": {strconv.Itoa(offset)},
	}
	escRaw, err := c.TableQuery(ctx, "sn_customerservice_escalation", escParams)
	if err != nil {
		return nil, err
	}
	var escData snEscalations
	if err := json.Unmarshal(escRaw, &escData); err != nil {
		return nil, fmt.Errorf("servicenow: decode escalations response: %w", err)
	}

	results := make([]EscalationDetail, 0, len(escData.Result))
	for _, item := range escData.Result {
		results = append(results, EscalationDetail{
			ID:          item.Number,
			EscalatedOn: item.OpenedAt,
			State:       escalationStateLabels[item.State],
			Severity:    item.EscalationSeverity,
		})
	}
	return results, nil
}

// EscalationRequest is the request body for EscalateCase — mirrors
// Ballerina EscalationRequest. RequestSource must be "Customer" or
// "Internal"; Reason must be "Inactivity", "Lack Of Progress", or "Customer
// Imposed Deadline"; Severity must be "High Severity" or "Medium Severity"
// (Ballerina enforces these via @constraint:String patterns on the request
// payload — validate the same set of values before calling EscalateCase and
// return 400 on mismatch, matching that constraint).
type EscalationRequest struct {
	Justification string `json:"justification"`
	RequestSource string `json:"requestSource"`
	Reason        string `json:"reason"`
	Severity      string `json:"severity"`
}

// EscalationResponse is EscalateCase's success response — mirrors Ballerina
// EscalationResponse.
type EscalationResponse struct {
	LinkedCaseNumber string `json:"linkedCaseNumber"`
	SysID            string `json:"sysId"`
}

// ErrEscalationConflict is returned by EscalateCase when the case has
// already been escalated.
var ErrEscalationConflict = errors.New("servicenow: case has already been escalated")

// escalationRequestSourceMap mirrors Ballerina utils.escalationRequestSourceMap.
var escalationRequestSourceMap = map[string]int{
	"Customer": 0,
	"Internal": 1,
}

// escalationReasonMap mirrors Ballerina utils.escalationReasonMap.
var escalationReasonMap = map[string]int{
	"Inactivity":                0,
	"Lack Of Progress":          1,
	"Customer Imposed Deadline": 2,
}

type snSysID struct {
	SysID string `json:"sys_id"`
}

type snSysIDResult struct {
	Result snSysID `json:"result"`
}

type snSysIDResultList struct {
	Result []snSysID `json:"result"`
}

// EscalateCase escalates caseNumber under accountNumber, or links it to an
// already-active escalation for that account. Mirrors Ballerina
// operations:escalateCase (its addEscalationGroups authorization check is
// the caller's responsibility — see handler/auth.go's
// requireViewerPermission). Returns ErrEscalationConflict when caseNumber is
// already linked to the account's active escalation.
//
// The active-escalation read and the create-new-escalation write below are
// serialized per accountSysID (see lockAccountEscalation): without it, two
// concurrent calls for the same account (a double-click, or two agents
// escalating cases on the same account together) can both read "no active
// escalation" and each create their own, leaving two open escalations with
// cases split between them and no fixed order for which one later lookups
// attach to. This only protects a single process -- ServiceNow itself gives
// no uniqueness guarantee here, so a multi-replica deployment would still
// need either a re-query-after-create reconciliation step or a ServiceNow-
// side uniqueness constraint to fully close this.
func (c *Client) EscalateCase(ctx context.Context, accountNumber, caseNumber string, request EscalationRequest, submittedByEmail string) (EscalationResponse, error) {
	if err := ValidateRecordNumberOrSysID(accountNumber); err != nil {
		return EscalationResponse{}, err
	}
	if err := ValidateRecordNumber(caseNumber); err != nil {
		return EscalationResponse{}, err
	}

	accountSysID, err := c.getAccountSysID(ctx, accountNumber)
	if err != nil {
		return EscalationResponse{}, err
	}
	caseSysID, err := c.getCaseSysIDByAccount(ctx, caseNumber, accountNumber)
	if err != nil {
		return EscalationResponse{}, err
	}

	unlock := c.lockAccountEscalation(accountSysID)
	defer unlock()

	activeEscalations, err := c.getActiveEscalationsForAccount(ctx, accountSysID)
	if err != nil {
		return EscalationResponse{}, err
	}

	if len(activeEscalations.Result) > 0 {
		escalationSysID := activeEscalations.Result[0].SysID
		escalated, err := c.isCaseEscalated(ctx, caseSysID, escalationSysID)
		if err != nil {
			return EscalationResponse{}, err
		}
		if escalated {
			return EscalationResponse{}, ErrEscalationConflict
		}
		return c.linkCaseToEscalation(ctx, caseSysID, escalationSysID, submittedByEmail)
	}

	escalationSysID, err := c.createNewEscalation(ctx, accountSysID, request)
	if err != nil {
		return EscalationResponse{}, err
	}
	return c.linkCaseToEscalation(ctx, caseSysID, escalationSysID, submittedByEmail)
}

func (c *Client) getAccountSysID(ctx context.Context, accountNumber string) (string, error) {
	raw, err := c.TableQuery(ctx, "customer_account", url.Values{
		"sysparm_query":  {"number=" + accountNumber},
		"sysparm_fields": {"sys_id"},
		"sysparm_limit":  {"1"},
	})
	if err != nil {
		return "", err
	}
	var data snSysIDResultList
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("servicenow: decode account sys_id response: %w", err)
	}
	if len(data.Result) == 0 {
		return "", ErrAccountNotFound
	}
	return data.Result[0].SysID, nil
}

func (c *Client) getCaseSysIDByAccount(ctx context.Context, caseNumber, accountNumber string) (string, error) {
	raw, err := c.TableQuery(ctx, "sn_customerservice_case", url.Values{
		"sysparm_query":  {BuildEncodedQuery("number="+caseNumber, "account.number="+accountNumber)},
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

func (c *Client) getActiveEscalationsForAccount(ctx context.Context, accountSysID string) (snSysIDResultList, error) {
	raw, err := c.TableQuery(ctx, "sn_customerservice_escalation", url.Values{
		"sysparm_query":  {BuildEncodedQuery("source_record="+accountSysID, "active=true")},
		"sysparm_fields": {"sys_id"},
		"sysparm_limit":  {"1"},
	})
	if err != nil {
		return snSysIDResultList{}, err
	}
	var data snSysIDResultList
	if err := json.Unmarshal(raw, &data); err != nil {
		return snSysIDResultList{}, fmt.Errorf("servicenow: decode active escalations response: %w", err)
	}
	return data, nil
}

func (c *Client) isCaseEscalated(ctx context.Context, caseSysID, escalationSysID string) (bool, error) {
	raw, err := c.TableQuery(ctx, "sn_customerservice_case", url.Values{
		"sysparm_query":  {BuildEncodedQuery("sys_id="+caseSysID, "active_account_escalation="+escalationSysID)},
		"sysparm_fields": {"sys_id"},
		"sysparm_limit":  {"1"},
	})
	if err != nil {
		return false, err
	}
	var data snSysIDResultList
	if err := json.Unmarshal(raw, &data); err != nil {
		return false, fmt.Errorf("servicenow: decode case escalation check response: %w", err)
	}
	return len(data.Result) > 0, nil
}

func (c *Client) linkCaseToEscalation(ctx context.Context, caseSysID, escalationSysID, submittedByEmail string) (EscalationResponse, error) {
	body, err := json.Marshal(map[string]string{
		"active_account_escalation": escalationSysID,
		"work_notes":                "[code] This Case has been escalated. <br><b>Submitted by: " + submittedByEmail + "</b><br>[/code]",
	})
	if err != nil {
		return EscalationResponse{}, fmt.Errorf("servicenow: encode escalation link request: %w", err)
	}

	raw, err := c.TablePatch(ctx, "sn_customerservice_case", caseSysID, body)
	if err != nil {
		return EscalationResponse{}, err
	}
	var data snEscalationResponseList
	if err := json.Unmarshal(raw, &data); err != nil {
		return EscalationResponse{}, fmt.Errorf("servicenow: decode escalation link response: %w", err)
	}
	return EscalationResponse{LinkedCaseNumber: data.Result.Number, SysID: data.Result.SysID}, nil
}

type snEscalationResponse struct {
	Number string `json:"number"`
	SysID  string `json:"sys_id"`
}

type snEscalationResponseList struct {
	Result snEscalationResponse `json:"result"`
}

func (c *Client) createNewEscalation(ctx context.Context, accountSysID string, request EscalationRequest) (string, error) {
	severitySysID, err := c.getEscalationSeveritySysID(ctx, request.Severity)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(map[string]any{
		"source_table":             "customer_account",
		"source_record":            accountSysID,
		"type":                     c.escalationTemplateID,
		"request_source":           escalationRequestSourceMap[request.RequestSource],
		"reason":                   escalationReasonMap[request.Reason],
		"escalation_justification": request.Justification,
		"escalation_severity":      severitySysID,
	})
	if err != nil {
		return "", fmt.Errorf("servicenow: encode escalation create request: %w", err)
	}

	raw, err := c.CustomPost(ctx, "/api/now/table/sn_customerservice_escalation", body)
	if err != nil {
		return "", err
	}
	var data snSysIDResult
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("servicenow: decode escalation create response: %w", err)
	}
	// A missing sys_id here (an unexpected response shape) would otherwise
	// have EscalateCase silently link the case to no escalation at all --
	// PATCHing active_account_escalation to empty and writing the "escalated"
	// work note while returning 200, with nothing actually escalated.
	if data.Result.SysID == "" {
		return "", errors.New("servicenow: escalation create response missing sys_id")
	}
	return data.Result.SysID, nil
}

func (c *Client) getEscalationSeveritySysID(ctx context.Context, severity string) (string, error) {
	if err := SanitizeQueryValue(severity); err != nil {
		return "", err
	}
	raw, err := c.TableQuery(ctx, "sn_customerservice_escalation_severity", url.Values{
		"sysparm_query":  {"name=" + severity},
		"sysparm_fields": {"sys_id"},
		"sysparm_limit":  {"1"},
	})
	if err != nil {
		return "", err
	}
	var data snSysIDResultList
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("servicenow: decode escalation severity response: %w", err)
	}
	if len(data.Result) == 0 {
		return "", fmt.Errorf("servicenow: no escalation severity found for %q", severity)
	}
	return data.Result[0].SysID, nil
}
