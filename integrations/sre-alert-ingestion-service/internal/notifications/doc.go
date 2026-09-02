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

// Package notifications is this service's escalation channel: a Twilio
// voice call, placed when a buffered alert has exhausted its retry budget
// against csm-integration-service. It exists specifically to notify SRE
// through a channel that does not itself depend on CSM's availability — the
// entire reason this service buffers and retries in the first place is to
// avoid being a single point of failure on CSM, and an escalation channel
// that also routed through CSM would defeat that.
//
// This package's shape (one Config/Client pair per channel, plain net/http
// rather than an SDK) matches the established convention at
// integrations/csm-notification-service/internal/notifications — that
// package already has a working Twilio integration (SendSMS + MakeCall) this
// service's twilio.go is adapted from, trimmed to MakeCall only since v1
// scope here is a single voice call to a static on-call number, not SMS.
package notifications
