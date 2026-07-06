// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
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
import ballerinax/azure_storage_service.files;

# Connection parameters for the Azure Storage Files client.
public type AzureFileStorageConfig record {|
    # Storage account name
    string accountName;
    # Account access key or SAS token string
    string accessKeyOrSAS;
    # Whether `accessKeyOrSAS` is a shared key or SAS
    files:AuthorizationMethod authorizationMethod;
|};
