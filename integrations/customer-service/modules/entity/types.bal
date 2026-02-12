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
import ballerina/constraint;
# Email Constraint.
@constraint:String {
    pattern: EMAIL_CONSTRAINT

}
public type EmailString string;

# Contact search payload.
public type ContactSearchPayload record {|
    # Email
    @constraint:String {
        pattern: EMAIL_CONSTRAINT
    }
    string? email = ();
    # Limit
    int? 'limit = DEFAULT_RECORD_LIMIT;
    # Offset
    int? offset = DEFAULT_RECORD_OFFSET;
|};

# [Entity] Account
public type Account record {|
    # Account ID (Salesforce)
    string? id;
    json...;
|};

# [Entity] Contact
public type Contact record {|
    # CRM id
    string? id;
    # Email of the customer
    string? email;
    # Account (Salesforce) related to the contact
    Account? account = ();
    json...;
|};
