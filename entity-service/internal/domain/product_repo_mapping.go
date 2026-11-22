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

package domain

// ProductRepoMapping is one row of product_repo_mapping: the GitHub
// repository an internal issue for that product is filed in.
type ProductRepoMapping struct {
	ProductName  string  `json:"productName"`
	Abbreviation *string `json:"abbreviation,omitempty"`
	Owner        string  `json:"owner"`
	Repository   string  `json:"repository"`
	GithubLabel  string  `json:"githubLabel"`
	// BoardProjectNumber is the GitHub project board issues for this product
	// are tracked on. Omitted when unset.
	BoardProjectNumber *int `json:"boardProjectNumber,omitempty"`
	// IsInternal is true when the repository is private to WSO2.
	IsInternal bool `json:"isInternal"`
}
