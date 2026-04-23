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

# Valid sort order values.
public enum SortOrder {
    ASC = "asc",
    DESC = "desc"
}

# Valid case sort field values.
public enum CaseSortField {
    CREATED_ON = "createdOn",
    UPDATED_ON = "updatedOn",
    SEVERITY = "severity",
    STATE = "state"
}

# Valid reference type values.
public enum ReferenceType {
    CASE = "case",
    CHANGE_REQUEST = "change_request",
    CONVERSATION = "conversation",
    DEPLOYMENT = "deployment"
}

# Valid comment type values.
public enum CommentType {
    COMMENTS = "comments"
}

# Valid conversation sort field values.
public enum ConversationSortField {
    CONVERSATION_CREATED_ON = "createdOn",
    CONVERSATION_UPDATED_ON = "updatedOn"
}

# Case type enum
public enum CaseType {
    DEFAULT_CASE = "default_case",
    SERVICE_REQUEST = "service_request",
    SECURITY_REPORT_ANALYSIS = "security_report_analysis",
    ANNOUNCEMENT = "announcement",
    ENGAGEMENT = "engagement"
}

# Time card state enum
public enum TimeCardState {
    APPROVED = "Approved"
}

# Stats filter enum
public enum StatsFilter {
    ME = "me"
}

# Valid product class values.
public enum ProductClass {
    PRODUCT_MODEL = "product_model"
}

# Valid product category values.
public enum ProductCategory {
    PRIVATE_DATA_PLANE = "pdp",
    CLOUD = "cl"
}
