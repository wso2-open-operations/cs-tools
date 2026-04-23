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

# WebSocket event type indicating the final response from the upstream AI chat agent.
const string EVENT_FINAL = "final";

# WebSocket event type indicating an error from the upstream AI chat agent.
const string EVENT_ERROR = "error";

# JSON key for the event type field in upstream WebSocket messages.
const string EVENT_TYPE_KEY = "type";

# JSON key for the payload field in the final upstream WebSocket event.
const string EVENT_PAYLOAD_KEY = "payload";
