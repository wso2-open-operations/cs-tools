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

package dispatch

import "strings"

// customerInProgressLabel is the one status customers see for what staff track
// as both Work In Progress and Waiting On WSO2. Customers act on Awaiting Info;
// the split between those two only says the case is with the support team, so
// showing it would read as a status change that did not happen. The customer
// portal shows the same merged state; the real states stay as they are for
// staff and in the database.
const customerInProgressLabel = "In Progress"

// isInProgressStatus reports whether a status label (display text or enum) is
// Work In Progress or Waiting On WSO2.
func isInProgressStatus(status string) bool {
	switch strings.Join(strings.Fields(strings.ReplaceAll(strings.ToLower(status), "_", " ")), " ") {
	case "work in progress", "waiting on wso2":
		return true
	}
	return false
}

// customerStatus is the status label a customer sees for a state.
func customerStatus(status string) string {
	if isInProgressStatus(status) {
		return customerInProgressLabel
	}
	return status
}

// customerSeesStatusChange reports whether moving from previous to next is a
// change a customer can see. A move between Work In Progress and Waiting On
// WSO2 is not, so they get no email for it. An unknown previous status (an
// older publisher) counts as a visible change, so no email is ever lost for
// lack of information.
func customerSeesStatusChange(previous, next string) bool {
	if strings.TrimSpace(previous) == "" {
		return true
	}
	return customerStatus(previous) != customerStatus(next)
}
