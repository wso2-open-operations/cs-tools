package domain

// MIRRORED FROM entity-service-plg/internal/domain/plg_entity_test.go.
//
// The two domain packages are hand-kept copies of each other — the BFF has no
// dependency on the slice, deliberately, so there is no way to share the type.
// That makes drift between them a live risk and an invisible one: each module
// compiles perfectly while disagreeing with the other about what a stage move
// or a health state means.
//
// Keeping the tests mirrored too is the cheap guard. If someone edits one
// package's rules and not the other, the copy of this file on the untouched
// side fails. Keep the two files identical apart from this header.

import "testing"

// The rules in this file are the ones written down in more than one place — in
// Go here, in SQL in migration 000016, and in TypeScript in the webapp. These
// tests pin the Go copy. They cannot prove the other two agree, so the ones
// that matter most are mirrored by TestSchemaMatchesDomain in the repository
// package, which asks the database the same questions.

func TestApplicablePlaybookTypes(t *testing.T) {
	// The worked example from the design discussion, kept verbatim because it
	// is the clearest statement of the rule anyone wrote:
	//
	//   pb1  stage=ACTIVATED  kind=PROGRESSIVE
	//   pb2  stage=ACTIVATED  kind=RECOVERY
	//   pb3  stage=ACTIVATED  kind=SUSTAINING
	//
	//   healthy at ACTIVATED  ->  pb1 and pb3 are addable
	//   at risk at ACTIVATED  ->  only pb2 is addable
	tests := []struct {
		health HealthState
		want   []PlaybookType
	}{
		{HealthHealthy, []PlaybookType{PlaybookProgressive, PlaybookSustaining}},
		{HealthAtRisk, []PlaybookType{PlaybookRecovery}},
	}

	for _, tc := range tests {
		got := ApplicablePlaybookTypes(tc.health)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %v, want %v", tc.health, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.health, got, tc.want)
				break
			}
		}
	}
}

// A healthy pairing must never be offered recovery work and an at-risk one must
// never be offered anything else. Stated as its own test because it is the
// property that matters, and it stays true if the order in the slice changes.
func TestApplicablePlaybookTypesArePartitioned(t *testing.T) {
	healthy := map[PlaybookType]bool{}
	for _, k := range ApplicablePlaybookTypes(HealthHealthy) {
		healthy[k] = true
	}
	for _, k := range ApplicablePlaybookTypes(HealthAtRisk) {
		if healthy[k] {
			t.Errorf("%s is offered to both healthy and at-risk pairings", k)
		}
	}
	// Every kind must be reachable. A kind nothing is ever offered is a kind an
	// author can create and no pairing can ever run.
	reachable := map[PlaybookType]bool{}
	for _, h := range []HealthState{HealthHealthy, HealthAtRisk} {
		for _, k := range ApplicablePlaybookTypes(h) {
			reachable[k] = true
		}
	}
	for _, k := range PlaybookTypeOrder {
		if !reachable[k] {
			t.Errorf("%s is never offered to any pairing", k)
		}
	}
}

// Adding a fourth kind without wording it is an easy thing to do and an
// invisible thing to get wrong: the UI falls back to the raw enum value and
// nobody notices until a screenshot.
func TestEveryPlaybookTypeIsLabelledAndValid(t *testing.T) {
	for _, k := range PlaybookTypeOrder {
		if PlaybookTypeLabel[k] == "" {
			t.Errorf("%s has no UI label", k)
		}
		if !ValidPlaybookType[k] {
			t.Errorf("%s is offered by the editor but rejected by validation", k)
		}
	}
	if len(ValidPlaybookType) != len(PlaybookTypeOrder) {
		t.Errorf("ValidPlaybookType has %d entries, PlaybookTypeOrder has %d — one was edited without the other",
			len(ValidPlaybookType), len(PlaybookTypeOrder))
	}
}

func TestCanMoveStage(t *testing.T) {
	tests := []struct {
		name     string
		from, to LifecycleStage
		want     bool
	}{
		{"one step forward", StageRegistration, StagePlgCsEligible, true},
		{"several steps forward", StageRegistration, StageCommercial, true},
		{"backwards", StageCommercial, StageRegistration, false},
		{"one step backwards", StagePlgCsEligible, StageRegistration, false},
		{"nowhere", StageActivated, StageActivated, false},
		{"out to abandoned from the first stage", StageRegistration, StageAbandoned, true},
		{"out to abandoned from the last", StageCommercial, StageAbandoned, true},
		{"abandoned is terminal", StageAbandoned, StageCommercial, false},
		{"abandoned to abandoned", StageAbandoned, StageAbandoned, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanMoveStage(tc.from, tc.to); got != tc.want {
				t.Errorf("CanMoveStage(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// Stated separately from the table above: whatever the stages are, a pairing
// must never be able to move to one that does not come after it. This keeps
// holding if a stage is inserted into the middle of the progression.
func TestNoStageMovesBackwards(t *testing.T) {
	for i, from := range StageProgression {
		for j, to := range StageProgression {
			want := j > i
			if got := CanMoveStage(from, to); got != want {
				t.Errorf("CanMoveStage(%s[%d], %s[%d]) = %v, want %v", from, i, to, j, got, want)
			}
		}
	}
}

func TestNextStage(t *testing.T) {
	for i, s := range StageProgression {
		next, ok := NextStage(s)
		if i == len(StageProgression)-1 {
			if ok {
				t.Errorf("the last stage %s reported a next stage %s", s, next)
			}
			continue
		}
		if !ok {
			t.Fatalf("%s reported no next stage", s)
		}
		if want := StageProgression[i+1]; next != want {
			t.Errorf("NextStage(%s) = %s, want %s", s, next, want)
		}
	}
	// ABANDONED is not on the progression at all, so nothing follows it.
	if next, ok := NextStage(StageAbandoned); ok {
		t.Errorf("NextStage(ABANDONED) = %s, want none", next)
	}
}

// ABANDONED must not appear in the progression: NextStage walks that slice, and
// including it would make ABANDONED the step that follows COMMERCIAL rather
// than an exit reachable from anywhere.
func TestAbandonedIsNotOnTheProgression(t *testing.T) {
	for _, s := range StageProgression {
		if s == StageAbandoned {
			t.Fatal("ABANDONED is on StageProgression — it is an exit, not a step")
		}
	}
	if !ValidLifecycleStage[StageAbandoned] {
		t.Error("ABANDONED is not a valid stage, but a pairing must be able to reach it")
	}
}

func TestTypeNeedsOptions(t *testing.T) {
	needs := map[TaskValueType]bool{
		ValueBoolean:      false,
		ValueString:       false,
		ValueNumber:       false,
		ValueChecklist:    true,
		ValueSingleSelect: true,
	}
	for _, vt := range TaskValueTypeOrder {
		want, known := needs[vt]
		if !known {
			t.Fatalf("task type %s was added without saying whether it needs options", vt)
		}
		if got := TypeNeedsOptions(vt); got != want {
			t.Errorf("TypeNeedsOptions(%s) = %v, want %v", vt, got, want)
		}
	}
	if len(ValidTaskValueType) != len(TaskValueTypeOrder) {
		t.Errorf("ValidTaskValueType has %d entries, TaskValueTypeOrder has %d",
			len(ValidTaskValueType), len(TaskValueTypeOrder))
	}
}

func TestWriteResultApplied(t *testing.T) {
	// The whole point of rowsAffected: zero means the precondition did not hold,
	// which the BFF turns into a 409 rather than a 500.
	if (WriteResult{RowsAffected: 0}).Applied() {
		t.Error("a write that affected no rows reported itself as applied")
	}
	if !(WriteResult{RowsAffected: 1}).Applied() {
		t.Error("a write that affected a row reported itself as not applied")
	}
}
