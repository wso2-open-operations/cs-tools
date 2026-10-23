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

package domain

// RotaImportSchemaVersion is the only version of the rota import file this service reads.
const RotaImportSchemaVersion = 1

// The two ways an import lands on the rota already there.
const (
	// RotaImportReplace clears every assignment of the rota's teams in the period first, and the person's time away on the days an entry is worked.
	RotaImportReplace = "replace"
	// RotaImportMerge writes the file's rows over what is there and keeps the rest; a clash the picker would refuse is refused.
	RotaImportMerge = "merge"
)

// ImportRotaRequest is the rota import file: a whole rota for a period, as one JSON document.
type ImportRotaRequest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Family        string                 `json:"family"`
	RotaCode      string                 `json:"rotaCode"`
	Period        ImportRotaPeriod       `json:"period"`
	Mode          string                 `json:"mode"`
	Assignments   []ImportRotaAssignment `json:"assignments"`
	Absences      []ImportRotaAbsence    `json:"absences"`
}

// ImportRotaPeriod is the span of rota days the file covers, both ends inclusive (YYYY-MM-DD).
type ImportRotaPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ImportRotaAssignment puts one engineer, named by email, on one window across a span of days.
type ImportRotaAssignment struct {
	Email     string  `json:"email"`
	TeamKey   string  `json:"teamKey"`
	ShiftCode string  `json:"shiftCode"`
	Tier      *string `json:"tier,omitempty"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Note      *string `json:"note,omitempty"`
}

// ImportRotaAbsence marks one engineer, named by email, away (or moved) across a span of days.
type ImportRotaAbsence struct {
	Email       string  `json:"email"`
	TeamKey     string  `json:"teamKey"`
	KindCode    string  `json:"kindCode"`
	AllocatedTo *string `json:"allocatedTo,omitempty"`
	From        string  `json:"from"`
	To          string  `json:"to"`
	Note        *string `json:"note,omitempty"`
}

// ImportRotaResponse says what the import did, or with dryRun what it would do.
type ImportRotaResponse struct {
	RotaCode string `json:"rotaCode"`
	From     string `json:"from"`
	To       string `json:"to"`
	Mode     string `json:"mode"`
	DryRun   bool   `json:"dryRun"`
	// Cleared is the assignments a replace removed before the file's rows went in.
	Cleared int `json:"cleared"`
	// Applied and Skipped count assignment days, as the roster's picker does.
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	// SkippedDays names, per assignment, the days its window is not worked.
	SkippedDays []ImportRotaSkippedDays `json:"skippedDays"`
	// AbsencesCreated is spans marked; AbsencesRemoved and AbsencesTrimmed are earlier spans they overlapped.
	AbsencesCreated int `json:"absencesCreated"`
	AbsencesRemoved int `json:"absencesRemoved"`
	AbsencesTrimmed int `json:"absencesTrimmed"`
	// LeaveWrittenOver is the earlier time away a replace removed, shortened or split under the file's assignments; a move is never one.
	LeaveWrittenOver int `json:"leaveWrittenOver"`
}

// ImportRotaSkippedDays is the days one assignment's window was not worked on.
type ImportRotaSkippedDays struct {
	Index int      `json:"index"`
	Dates []string `json:"dates"`
}
