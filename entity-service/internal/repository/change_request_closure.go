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

package repository

import "fmt"

// changeRequestClosureSets is the SET list that records when a change request
// was completed and by whom: change_request.closed_on and closed_by_user_id,
// stamped when the UPDATE moves the row INTO Closed or Rollback (from any other
// state, or none). A change request is completed either way: Closed is a
// success, Rollback a failure, and both are final (change_request_transitions.go),
// so nothing ever clears the stamp. Canceled is not a completion and stamps
// nothing. A resend of the state the row is already in moves nothing and keeps
// the first stamp. The same transition-gated CASE shape as case_repo.go's
// closed_on / closed_by_user_id.
//
// stateParam is the placeholder of the state the UPDATE writes ("$2");
// actorEmailParam the placeholder of the actor's email, or "" when the writer is
// no person (the GitHub sync): the actor's "user" id is looked up by email the
// way userIDByEmailSQL does, and an email with no user records no closer.
func changeRequestClosureSets(stateParam, actorEmailParam string) string {
	entering := fmt.Sprintf(
		"%s::change_request_state_enum IN ('CLOSED', 'ROLLBACK') AND (state IS NULL OR state NOT IN ('CLOSED', 'ROLLBACK'))",
		stateParam)
	closer := "NULL::uuid"
	if actorEmailParam != "" {
		closer = fmt.Sprintf(`(SELECT u.id FROM "user" u WHERE LOWER(u.email) = LOWER(%s::text) ORDER BY u.id LIMIT 1)`, actorEmailParam)
	}
	return fmt.Sprintf(
		"closed_on = CASE WHEN %[1]s THEN NOW() ELSE closed_on END, "+
			"closed_by_user_id = CASE WHEN %[1]s THEN %[2]s ELSE closed_by_user_id END",
		entering, closer)
}
