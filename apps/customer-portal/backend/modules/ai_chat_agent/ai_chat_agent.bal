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

import ballerina/websocket;
import ballerina/log;
# Create case classification for the given payload.
#
# + payload - Case classification payload
# + return - Case classification response or error
public isolated function createCaseClassification(CaseClassificationPayload payload)
    returns CaseClassificationResponse|error {

    return aiChatAgentClient->/case_classification.post(payload);
}

# Create a chat for the given payload.
# 
# + projectId - Project ID
# + conversationId - Conversation ID
# + payload - Conversation payload
# + return - Chat response or error
public isolated function createChat(string projectId, string conversationId, ConversationPayload payload)
    returns ChatResponse|error {

    ChatPayload chatPayload = {
        message: payload.message,
        accountId: projectId,
        conversationId: conversationId,
        envProducts: payload.envProducts
    };
    return aiChatAgentClient->/chat.post(chatPayload);
}

# List conversations for the given project ID.
# 
# + projectId - Project ID
# + return - List of conversations or error
public isolated function listConversations(string projectId) returns ConversationListResponse|error {
    return aiChatAgentClient->/chat/conversations/[projectId];
}

# Get chat history.
# 
# + projectId - Project ID
# + conversationId - Conversation ID
# + return - Chat history response or error
public isolated function getChatHistory(string projectId, string conversationId) returns ChatHistoryResponse|error {
    return aiChatAgentClient->/chat/history/[projectId]/[conversationId];
}

# Delete chat conversation.
# 
# + projectId - Project ID
# + conversationId - Conversation ID
# + return - Success message or error
public isolated function deleteChatConversation(string projectId, string conversationId) returns DeleteConversationResponse|error {
    return aiChatAgentClient->/chat/history/[projectId]/[conversationId].delete();
}

# Get recommendation for user query.
# 
# + payload - Recommendation payload
# + return - Recommendation response or error
public isolated function getRecommendation(RecommendationRequest payload) returns RecommendationResponse|error {
    return aiChatAgentClient->/recommendations.post(payload);
}

# Get summary for a conversation.
#
# + projectId - Project ID
# + conversationId - Conversation ID
# + return - Summary response or error
public isolated function getSummary(string projectId, string conversationId) returns ConversationSummaryResponse|error {
    return aiChatAgentClient->/chat/summary/[projectId]/[conversationId];
}

# Stream chat events from the upstream AI chat agent WebSocket back to the browser caller.
# Opens a dedicated upstream connection per call, sends the payload, then pipes every event
# verbatim until a "final" or "error" event or the upstream connection closes.
#
# + sessionId - Conversation/session ID used to route to the upstream Python session
# + payload - Raw JSON string (user_message) to forward to the upstream agent
# + caller - The browser WebSocket caller to forward events back to
# + return - The final event payload as a map of JSON for further processing, or error
public isolated function streamChat(string sessionId, string payload, websocket:Caller caller) returns map<json>|error {
    websocket:Client agentClient = check createAiChatAgentWsClient(sessionId);
    check agentClient->writeTextMessage(payload);
    boolean upstreamClosed = false;
    map<json> finalPayload = {};
    while true {
        string|error event = agentClient->readTextMessage();
        if event is error {
            if event is websocket:ConnectionClosureError {
                upstreamClosed = true;
            } else {
                log:printError("Error reading from upstream AI chat agent", event);
                json errorPayload = {"type": "error", "message": event.message()};
                error? writeErr = caller->writeTextMessage(errorPayload.toJsonString());
                if writeErr is error {
                    log:printError("Failed to send error to caller (client disconnected)", writeErr);
                }
            }
            break;
        }
        error? writeErr = caller->writeTextMessage(event);
        if writeErr is error {
            log:printError("Failed to forward event to caller (client disconnected)", writeErr);
            break;
        }
        json|error parsed = event.fromJsonString();
        if parsed is error {
            log:printError("Failed to parse upstream event as JSON", parsed);
        }
        if parsed is map<json> {
            string evtType = (parsed[EVENT_TYPE_KEY] ?: "").toString();
            if evtType == EVENT_FINAL {
                json eventPayload = parsed[EVENT_PAYLOAD_KEY] ?: parsed;
                finalPayload = eventPayload is map<json> ? eventPayload : parsed;
                break;
            }
            if evtType == EVENT_ERROR {
                break;
            }
        }
    }
    if !upstreamClosed {
        error? closeErr = agentClient->close(1000, "session complete");
        if closeErr is error {
            log:printError("Failed to close upstream WebSocket connection", closeErr);
        }
    }
    return finalPayload;
}
