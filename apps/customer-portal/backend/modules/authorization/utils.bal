// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

# Helper function to check if user has roles.
#
# + requiredRoles - Required Role list
# + userRoles - Roles list, The user has
# + return - Allow or not
# TODO: Use this function after roles and groups are finalized.
public isolated function checkRoles(string[] requiredRoles, readonly & string[] userRoles) returns boolean =>
    requiredRoles.every(reqRole => userRoles.indexOf(reqRole) !is ());
