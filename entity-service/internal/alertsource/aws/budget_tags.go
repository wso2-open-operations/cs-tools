// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package aws

import "context"

// BudgetTagFetcher reads the tags of an AWS Budget by account id and budget name; the bridge runs without one, so budget alerts keep their configured values.
type BudgetTagFetcher interface {
	Tags(ctx context.Context, accountID, budgetName string) (map[string]string, error)
}
