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

# WebSocket event acknowledging a stored answer rating.
const string EVENT_FEEDBACK_ACK = "feedback_ack";

# WebSocket event acknowledging a token-increase request.
const string EVENT_TOKEN_REQUEST_ACK = "token_request_ack";

# Inbound message type carrying a thumbs up/down on an answer.
public const string MSG_TYPE_FEEDBACK = "feedback";

# Inbound message type asking support to raise a token limit.
public const string MSG_TYPE_TOKEN_INCREASE_REQUEST = "token_increase_request";

# How long to wait for a side-channel acknowledgement before giving up.
#
# The upstream answers these in well under a second. A bound matters because the
# alternative is an upstream connection parked indefinitely: the websocket client
# is otherwise created with no read timeout at all.
const decimal SIDE_CHANNEL_READ_TIMEOUT = 30.0;
