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

import customer_portal.reasoning_filter;

import ballerina/log;
import ballerina/websocket;
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
    ChatResponse|error response = aiChatAgentClient->/chat.post(chatPayload);
    if response is ChatResponse {
        // The agent can put its <thinking> reasoning inside the answer. Callers
        // persist this reply as a conversation comment, hand it to the recommender
        // and return it to the browser, so it is removed here, once, before any of
        // them see it.
        response.message = reasoning_filter:stripThinkingBlocks(response.message);
    }
    return response;
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
# verbatim until a "final" or "error" event or the upstream connection closes. The one thing
# not forwarded verbatim is the answer text of the "final" event: any <thinking> reasoning the
# agent put in it is removed first (see reasoning_filter:stripThinkingBlocks), both from the frame the browser
# receives and from the payload returned for the caller to persist.
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
        json|error parsed = event.fromJsonString();
        if parsed is error {
            log:printError("Failed to parse upstream event as JSON", parsed);
        }
        string evtType = parsed is map<json> ? (parsed[EVENT_TYPE_KEY] ?: "").toString() : "";

        string frame = event;
        map<json> finalFields = {};
        if parsed is map<json> && evtType == EVENT_FINAL {
            [string, map<json>] scrubbed = reasoning_filter:scrubFinalEvent(parsed, event);
            frame = scrubbed[0];
            finalFields = scrubbed[1];
        }
        error? writeErr = caller->writeTextMessage(frame);
        if writeErr is error {
            log:printError("Failed to forward event to caller (client disconnected)", writeErr);
            break;
        }
        if evtType == EVENT_FINAL && parsed is map<json> {
            finalPayload = finalFields;
            break;
        }
        if evtType == EVENT_ERROR {
            break;
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

# Forward a side-channel message — an answer rating, or a token-increase request —
# to the upstream agent and pipe events back until it acknowledges.
#
# Deliberately separate from streamChat. These are not chat turns: the agent
# answers them with a "*_ack" and never sends a "final". Routing them through
# streamChat left it reading for an event that could not arrive, holding the
# caller's streaming lock until the connection gave out — so the customer's next
# real message was rejected with "A response is already being streamed" and never
# forwarded upstream at all. The rating persisted; the conversation was dead.
#
# + sessionId - Conversation/session ID used to route to the upstream Python session
# + payload - Raw JSON string (feedback or token_increase_request) to forward
# + caller - The browser WebSocket caller to forward the acknowledgement back to
# + return - Error if the upstream connection could not be opened or written to
public isolated function sendSideChannelMessage(string sessionId, string payload,
        websocket:Caller caller) returns error? {
    websocket:Client agentClient = check createAiChatAgentWsClient(sessionId, SIDE_CHANNEL_READ_TIMEOUT);
    // Not `check`: returning straight out of here would leave the connection we
    // just opened dangling, which is the very thing this function exists to stop.
    error? upstreamWriteErr = agentClient->writeTextMessage(payload);
    if upstreamWriteErr is error {
        closeUpstream(agentClient);
        return upstreamWriteErr;
    }
    boolean upstreamClosed = false;
    while true {
        string|error event = agentClient->readTextMessage();
        if event is error {
            if event is websocket:ConnectionClosureError {
                upstreamClosed = true;
            } else {
                // Includes the read timeout. The rating may well have been stored
                // — the write succeeded — so this is logged, not surfaced as a
                // failure to the customer.
                log:printError("Error reading acknowledgement from upstream AI chat agent", event);
            }
            break;
        }
        error? writeErr = caller->writeTextMessage(event);
        if writeErr is error {
            log:printError("Failed to forward acknowledgement to caller (client disconnected)", writeErr);
            break;
        }
        json|error parsed = event.fromJsonString();
        if parsed is error {
            log:printError("Failed to parse upstream event as JSON", parsed);
            continue;
        }
        if parsed is map<json> {
            string evtType = (parsed[EVENT_TYPE_KEY] ?: "").toString();
            if evtType == EVENT_FEEDBACK_ACK || evtType == EVENT_TOKEN_REQUEST_ACK
                || evtType == EVENT_ERROR {
                break;
            }
        }
    }
    if !upstreamClosed {
        closeUpstream(agentClient);
    }
    return;
}

# Close a side-channel upstream connection, logging rather than raising if the
# close itself fails. Every exit path from sendSideChannelMessage goes through
# here, and each reaches it at most once, so the connection is always released
# and never closed twice.
#
# + agentClient - The upstream AI chat agent connection to release
isolated function closeUpstream(websocket:Client agentClient) {
    error? closeErr = agentClient->close(1000, "acknowledged");
    if closeErr is error {
        log:printError("Failed to close upstream WebSocket connection", closeErr);
    }
}
