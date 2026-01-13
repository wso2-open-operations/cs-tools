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
import customer_portal.entity;

# Cache configuration record.
public type CacheConfig record {|
    # Maximum number of entries in cache
    int capacity = 500;
    # Default maximum age of cache entries in seconds
    decimal defaultMaxAge = 3600;
    # Eviction factor for cache cleanup
    float evictionFactor = 0.2;
    # Cleanup interval in seconds
    decimal cleanupInterval = 1800.0;
|};

# Case search filters.
public type CaseSearchFilters record {|
    # Status ID
    int status?;
    # Severity ID
    int severity?;
    # Deployment ID
    string deployment?;
|};

# Payload for case search.
public type CaseSearchPayload record {|
    # List of case types to filter
    string[] caseTypes?;
    # Filter criteria
    CaseSearchFilters filters?;
    # Sort configuration
    entity:SortBy sortBy?;
    # Pagination details
    entity:Pagination pagination?;
|};
