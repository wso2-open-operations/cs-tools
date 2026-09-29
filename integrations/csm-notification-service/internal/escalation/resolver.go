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

package escalation

import "context"

// Shift is the "effective shift" at the time an incident is reported. Section
// 5.0's rule table selects recipients from the incident's own attributes plus
// this.
type Shift string

const (
	ShiftLK         Shift = "LK"          // 9AM - 6PM IST
	ShiftLKMorning  Shift = "LK_MORNING"  // 6-9 AM IST
	ShiftLKEvening  Shift = "LK_EVENING"  // 6-9 PM IST
	ShiftLKWeekend  Shift = "LK_WEEKEND"  // 6AM - 9PM IST
	ShiftUSA        Shift = "USA"         // 9PM - 6AM IST
	ShiftUSAWeekend Shift = "USA_WEEKEND" // 9PM - 6AM IST, weekends
)

// IsRotation reports whether this shift is a rotation at all. The regular LK
// and USA business-hours shifts are not, and never carry a notification level.
//
// A rotation is necessary but not sufficient for LEVEL_0 to exist — see
// RoutingContext.HasNotificationLevel, which is what BuildPlan actually asks.
func (s Shift) IsRotation() bool {
	switch s {
	case ShiftLKMorning, ShiftLKEvening, ShiftLKWeekend, ShiftUSAWeekend:
		return true
	default:
		return false
	}
}

// RoutingContext is the full input to section 5.0's rule table (R1–R14). Every
// field is drawn from the incident record except Shift, which is derived from
// when the incident was reported.
type RoutingContext struct {
	// Product is the WSO2 product on the incident; empty when absent, which
	// rules R7, R8, R13 and R14 route on explicitly (and which section 12.0
	// treats as an erroneous scenario worth emailing about).
	Product string
	// ABTEligible is whether the account qualifies for ABT-based support,
	// determined by the assigned product.
	//
	// A POINTER, because there are three states and not two. The rule table
	// routes an ABT-eligible incident down one set of rows (R1-R4, R9, R10)
	// and a sub-team one down another (R5, R6, R11, R12), so "we were not
	// told" is a genuinely different situation from "told no" — and it is the
	// common one today, since no publisher populates the field. A plain bool
	// made every such incident indistinguishable from an explicit sub-team
	// answer, silently routing all of them down the R5/R6/R11/R12 rows and
	// giving a USA_WEEKEND incident a LEVEL_0 that rule R10 says it should
	// not have. Nil says so instead; see Rule and HasNotificationLevel for
	// what each does about it.
	ABTEligible *bool
	// AssignedCRETeam is the CS team on the incident; empty when unassigned,
	// which rules R3 and R4 route on (and section 12.0 also flags).
	AssignedCRETeam string
	// Shift is the effective shift when the incident was reported.
	Shift Shift
}

// HasNotificationLevel reports whether LEVEL_0 exists for this incident.
//
// Section 3.0 makes the notification level a rotation-only step, but the shift
// alone does not decide it: section 6.0's matrix and the section 5.0 rule table
// disagree about USA_WEEKEND depending on the business unit.
//
//   - R10 (product present, ABT-eligible, USA_WEEKEND) lists LEVEL_1..LEVEL_4
//     only — no notification level.
//   - R12 (product present, NOT ABT-eligible, USA_WEEKEND) and R14 (no product,
//     USA_WEEKEND) both list "Level 0: Rota Members".
//
// Section 6.0's matrix agrees: on USA_WEEKEND the INTEGRATION row has neither a
// rotation-member nor a rotation-lead tick, while the IAM row has one. ABT
// eligibility is what separates them — the Integration/APIM BU is the
// ABT-eligible side (see section 8.0's two team models), so an ABT-eligible
// incident on the USA weekend rotation starts at LEVEL_1.
//
// Every other rotation (LK_MORNING, LK_EVENING, LK_WEEKEND) has a notification
// level for both business units; only the recipients differ, which is the
// Resolver's concern rather than this one's.
func (rc RoutingContext) HasNotificationLevel() bool {
	if !rc.Shift.IsRotation() {
		return false
	}
	// USA_WEEKEND is the one rotation whose notification level depends on the
	// business unit. With eligibility unknown, the level is included: the two
	// errors are not symmetric. Including it wrongly wakes a rotation member
	// who was not on the hook, and the ladder still reaches every later rung
	// on time. Omitting it wrongly removes the first and fastest rung from an
	// unattended incident on a weekend night — the very situation the
	// notification level exists for. Paging one person too many is the
	// recoverable direction.
	if rc.Shift == ShiftUSAWeekend && rc.isABTEligible() {
		return false
	}
	return true
}

// isABTEligible reports a definite yes. Unknown is not yes.
func (rc RoutingContext) isABTEligible() bool {
	return rc.ABTEligible != nil && *rc.ABTEligible
}

// abtKnown reports whether the publisher told us either way.
func (rc RoutingContext) abtKnown() bool { return rc.ABTEligible != nil }

// ABTEligibility renders the three states for a log line or a work note,
// where "unknown" is the part worth seeing.
func (rc RoutingContext) ABTEligibility() string {
	if !rc.abtKnown() {
		return "unknown"
	}
	if *rc.ABTEligible {
		return "yes"
	}
	return "no"
}

// Rule identifies which row of section 5.0's rule table (R1–R14) this incident
// routes by. It is the "path" a call alert took: the table's four inputs —
// is a WSO2 product present, is the account ABT-eligible, is a CRE team
// assigned, and the effective shift — select exactly one row, and that row is
// what decides who each level resolves to.
//
// Nothing in the ladder branches on this; the Resolver already applies the same
// inputs directly. It exists so the decision is *reportable*: an operator
// asking "why did this page the Americas leads and not ours" gets an answer
// from one log line and the work note, instead of re-deriving the table by
// hand from four fields.
//
// The rows, as the document orders them:
//
//	           product  ABT  team   shift
//	R1         yes      yes  yes    LK                  R9   yes  yes  n/a  USA
//	R2         yes      yes  yes    LK rotations        R10  yes  yes  n/a  USA_WEEKEND
//	R3         yes      yes  no     LK                  R11  yes  no   n/a  USA
//	R4         yes      yes  no     LK rotations        R12  yes  no   n/a  USA_WEEKEND
//	R5         yes      no   n/a    LK                  R13  no   n/a  n/a  USA
//	R6         yes      no   n/a    LK rotations        R14  no   n/a  n/a  USA_WEEKEND
//	R7         no       n/a  n/a    LK
//	R8         no       n/a  n/a    LK rotations
//
// "LK rotations" is LK_MORNING, LK_EVENING or LK_WEEKEND. An unrecognised
// shift yields "UNKNOWN" rather than a guess: the table has no row for one,
// and quietly reporting the wrong path is worse than admitting there is none.
func (rc RoutingContext) Rule() string {
	lkRotation := rc.Shift == ShiftLKMorning || rc.Shift == ShiftLKEvening || rc.Shift == ShiftLKWeekend

	// No product at all — section 12.0 treats this as an erroneous scenario
	// worth emailing about, and the table routes it to a shift-wide pool.
	if rc.Product == "" {
		switch {
		case rc.Shift == ShiftLK:
			return "R7"
		case lkRotation:
			return "R8"
		case rc.Shift == ShiftUSA:
			return "R13"
		case rc.Shift == ShiftUSAWeekend:
			return "R14"
		}
		return "UNKNOWN"
	}

	// Eligibility splits the table in half — the ABT rows from the sub-team
	// ones — so without it the row genuinely cannot be named. Saying so is
	// the point: this string is what a log line and the incident's own work
	// note report as the path a call alert took, and a confident "R12" on an
	// incident nobody classified is worse than an honest admission, because
	// it reads as a decision that was made rather than one that was missed.
	if !rc.abtKnown() {
		return "UNKNOWN_ABT"
	}

	if *rc.ABTEligible {
		switch {
		case rc.Shift == ShiftUSA:
			return "R9"
		case rc.Shift == ShiftUSAWeekend:
			return "R10"
		case rc.Shift == ShiftLK && rc.AssignedCRETeam != "":
			return "R1"
		case rc.Shift == ShiftLK:
			return "R3"
		case lkRotation && rc.AssignedCRETeam != "":
			return "R2"
		case lkRotation:
			return "R4"
		}
		return "UNKNOWN"
	}

	// Not ABT-eligible: the sub-team model, where the table stops consulting
	// whether a CRE team is assigned at all ("n/a" in every one of these rows).
	switch {
	case rc.Shift == ShiftLK:
		return "R5"
	case lkRotation:
		return "R6"
	case rc.Shift == ShiftUSA:
		return "R11"
	case rc.Shift == ShiftUSAWeekend:
		return "R12"
	}
	return "UNKNOWN"
}

// Recipient is one person to call or email at a level.
type Recipient struct {
	Email string
	Name  string
	// Phone is empty when the user profile has no mobile number. The
	// specification's execution summary logs that case as
	// [LEVEL_n][ERROR][NO_NUMBER][email] and carries on — a missing number
	// must never abort a level.
	Phone string
}

// Resolver turns a level plus routing context into the people to contact.
//
// NOT IMPLEMENTED YET, deliberately. The specification resolves recipients
// from organisation data that this platform does not hold: ServiceNow's
// sys_user_group_type (BU/shift tags per team), u_team_member_role (the
// escalation roles — sub lead, team lead, BU head, head of CRE) and the
// On-Call Scheduling module (effective shift and escalation paths). See the
// administrative guide for how each is configured today.
//
// Two of those have partial equivalents here already — CSM_TEAM_REGISTRY holds
// teams with a FAMILY, and the rotations tables hold who is on shift — but
// neither carries the BU/shift tags or the escalation roles. Closing that gap
// is its own piece of work; this interface is the seam it plugs into, so the
// scheduling half above can be built, reviewed and tested independently.
//
// Availability filtering applies only at LEVEL_0 (section 8.0): every other
// level is contacted regardless of whether they are on shift.
type Resolver interface {
	// Resolve returns the recipients for one level. An empty slice is valid
	// and means "nobody at this level" — the caller logs it and moves on to
	// the next level rather than failing.
	Resolve(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error)
}
