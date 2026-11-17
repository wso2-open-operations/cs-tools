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

package entity

import (
	"context"
	"net/http"
)

// PreviewSalesforceResync calls POST /salesforce/resync/preview on the entity
// service: a read-only comparison of one account's or project's Salesforce data
// with the database. The body is passed through unchanged and the response is
// returned as raw JSON. It runs on the client's shared timeout
// (ENTITY_SERVICE_TIMEOUT); see the note on SalesforceResyncHandler.
func (c *CustomerEntityClient) PreviewSalesforceResync(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/salesforce/resync/preview", body)
}

// ApplySalesforceResync calls POST /salesforce/resync/apply on the entity
// service: overwrites the chosen components from Salesforce. The body is passed
// through unchanged and the response is returned as raw JSON.
func (c *CustomerEntityClient) ApplySalesforceResync(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/salesforce/resync/apply", body)
}
