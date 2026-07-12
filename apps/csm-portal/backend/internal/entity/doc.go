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

// Package entity groups the clients for every upstream "entity" service used
// by the CSM portal backend. Each one gets its own config/client pair in its
// own file, since they are distinct, separately-deployed services:
//
//   - customer.go / customer_client.go: CustomerEntityConfig/CustomerEntityClient
//     — this repo's entity-service (cases, accounts, projects, products,
//     deployments, users, incidents, etc.)
//   - engineering.go: EngineeringEntityConfig/EngineeringEntityClient — the
//     wso2-enterprise/digiops-engineering entity service, used to create
//     GitHub issues in engineering repos.
package entity
