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

package holidays

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// employeeLocationQuery is the leave app's getEmployee query, cut to the one
// field the Team Schedule needs.
const employeeLocationQuery = `query getEmployee($email: String!) { employee(email: $email) { location } }`

// ErrEmployeeNotFound is returned when the employee service knows no
// employee for the email.
var ErrEmployeeNotFound = errors.New("holidays: employee not found")

// EmployeeClient reads an employee's location from the HR entity GraphQL
// service.
type EmployeeClient struct {
	baseURL string
	tokens  *tokenSource
}

// NewEmployeeClient builds an EmployeeClient for the HR entity GraphQL service
// at baseURL (the GraphQL endpoint itself).
func NewEmployeeClient(baseURL string, creds Credentials) *EmployeeClient {
	return &EmployeeClient{baseURL: strings.TrimRight(baseURL, "/"), tokens: newTokenSource(creds)}
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type employeeLocationResponse struct {
	Data struct {
		Employee *struct {
			Location *string `json:"location"`
		} `json:"employee"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Location returns the employee's location (e.g. "Sri Lanka"), "" when the
// employee has none recorded, or ErrEmployeeNotFound.
func (c *EmployeeClient) Location(ctx context.Context, email string) (string, error) {
	payload, err := json.Marshal(graphQLRequest{Query: employeeLocationQuery, Variables: map[string]any{"email": email}})
	if err != nil {
		return "", fmt.Errorf("holidays: marshal employee query: %w", err)
	}
	raw, err := c.tokens.do(ctx, "employee service", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payload))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, err
	})
	if err != nil {
		return "", err
	}
	var resp employeeLocationResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("holidays: parse employee response: %w", err)
	}
	if resp.Data.Employee == nil {
		if len(resp.Errors) > 0 {
			// The message is the service's own; it never carries the email
			// back here, but keep it out of the error anyway.
			return "", fmt.Errorf("holidays: employee query failed (%d errors)", len(resp.Errors))
		}
		return "", ErrEmployeeNotFound
	}
	if resp.Data.Employee.Location == nil {
		return "", nil
	}
	return strings.TrimSpace(*resp.Data.Employee.Location), nil
}
