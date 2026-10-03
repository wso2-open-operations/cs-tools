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

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is how the two escalation ladders behave, read from a file rather
// than from the environment.
//
// It is a file because every knob in it changes how many telephone calls get
// placed, and therefore what the ladder costs. Turning a priority off, capping
// a ladder at LEVEL_2, or switching a deployment to the log channel are all
// things somebody needs to do between incidents, without waiting for a build.
// Spreading that across eight environment variables -- which is where it
// started -- means the answer to "what will this deployment actually dial" is
// assembled by hand from a container spec.
//
// Both ladders are configured here, not just the CRE one. They share an
// engine, they share a Twilio account, and they share a bill.
type Config struct {
	// Enabled is the master switch across both ladders. It exists so that
	// "escalation is deliberately off" is a thing a deployment can say.
	// Until this file existed the only way to stop the ladder was to withhold
	// a roster or an entity-service URL, which is indistinguishable from
	// having misconfigured it -- the logs said the same thing either way.
	Enabled bool `yaml:"enabled"`

	// CRE and SRE are configured separately because they are answerable to
	// different people and fail for different reasons. One can be stopped
	// without stopping the other.
	CRE LadderConfig `yaml:"cre"`
	SRE LadderConfig `yaml:"sre"`
}

// LadderConfig is one ladder's behaviour.
type LadderConfig struct {
	Enabled bool      `yaml:"enabled"`
	Channel Channel   `yaml:"channel"`
	Start   StartWhen `yaml:"trigger"`
	Safety  Safety    `yaml:"safety"`
	// Teams names the teams the rules refer to by role rather than by name --
	// the ABTs, the Americas team, the leadership team.
	Teams TeamKeys `yaml:"teams"`
	// Heads are the last two rungs, named outright.
	Heads Heads `yaml:"heads"`
	// AlertDuty is how the nominated rungs are read.
	AlertDuty AlertDuty `yaml:"alertDuty"`
	// Acknowledgement decides what stops a ladder.
	Acknowledgement Acknowledgement `yaml:"acknowledgement"`
	// Rules is the escalation rule table. Empty uses DefaultRules, which is
	// the shipped transcription of the spreadsheet; a deployment overrides a
	// row here rather than waiting for a release.
	Rules []Rule `yaml:"rules"`
}

// StartWhen decides which incidents get a ladder at all.
//
// Every list is an allowlist, and an empty list means "no opinion", not "none"
// -- so a file that sets only `enabled: true` behaves exactly as the service
// did before this type existed. A deployment narrows from there.
type StartWhen struct {
	// Priorities are the priority codes that get a ladder (S0..S4). Empty
	// means every priority that has a policy row. Dropping S3 and S4 here is
	// the bluntest cost control available and needs no other change.
	Priorities []string `yaml:"priorities"`
	// Teams is an assignment-group allowlist; empty means any team.
	Teams []string `yaml:"teams"`
	// ExcludeTeams wins over Teams, so a single noisy group can be silenced
	// without rewriting the allowlist.
	ExcludeTeams []string `yaml:"excludeTeams"`
	// Shifts restricts the ladder to certain shifts, for a deployment that
	// wants (say) only the Americas night covered while the rest is trialled.
	Shifts []string `yaml:"shifts"`
	// RequireKnownTeam refuses to start a ladder for an incident whose team
	// cannot be resolved, instead of guessing.
	//
	// The default without this was to fall through to the CRE ladder, which
	// means an SRE incident whose group could not be placed woke CRE
	// engineers -- and logged nothing at all in one of the three paths that
	// could produce it. Refusing is louder and cheaper than calling the wrong
	// people.
	RequireKnownTeam bool `yaml:"requireKnownTeam"`
}

// Heads names the two people the top of the ladder reaches.
//
// They are configuration rather than a team lookup because they are not a
// team: there is one CRE head and one CS head, they change rarely, and the
// ladder's only interest in them is a number to call. Inventing a
// "cre-leadership" team to hold two rows made the schema carry an org chart it
// otherwise has no opinion about, and made the top of the ladder depend on a
// team key being spelled right in two places.
//
// A head left empty falls back to the old lookup -- role cre_head or cs_head
// inside teams.leadership -- so an existing deployment keeps working.
type Heads struct {
	CRE Person `yaml:"cre"`
	CS  Person `yaml:"cs"`
}

// Person is somebody the ladder can reach.
type Person struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
	// Phone is E.164. Only the call channel needs it.
	Phone string `yaml:"phone"`
}

// Set reports whether this person is worth calling.
func (p Person) Set() bool {
	return strings.TrimSpace(p.Email) != "" || strings.TrimSpace(p.Phone) != ""
}

// AlertDuty is how the nominated rungs are read.
type AlertDuty struct {
	// Tiers are the nominations that exist, in the order a rung walks them.
	// Empty means T1, T2, T3.
	Tiers []string `yaml:"tiers"`
	// PerTeam is how many nominees a rung takes from each team it spans.
	// Zero means all of them.
	//
	// It is configurable because the two readings of the sheet differ by
	// exactly this number: "the ABT's nominees" is three, "one nominee from
	// each ABT" is one each. Changing which is right is a number here, not a
	// release.
	PerTeam int `yaml:"perTeam"`
}

// AlertTiers is the tier vocabulary this ladder reads.
func (l LadderConfig) AlertTiers() []string {
	if len(l.AlertDuty.Tiers) == 0 {
		return []string{"T1", "T2", "T3"}
	}
	return l.AlertDuty.Tiers
}

// Acknowledgement is what counts as somebody having picked the incident up.
type Acknowledgement struct {
	// RequireBoth means a ladder stops only once the incident has BOTH moved
	// out of NEW and received a public comment. Defaults to true.
	//
	// A pointer so absent and false are different: the default is the stricter
	// rule, and a deployment that wants either gesture alone has to say so
	// rather than get it by omitting a key.
	RequireBoth *bool `yaml:"requireBoth"`
}

// RequireBothGestures reports whether acknowledgement takes both gestures.
func (l LadderConfig) RequireBothGestures() bool {
	if l.Acknowledgement.RequireBoth == nil {
		return true
	}
	return *l.Acknowledgement.RequireBoth
}

// Safety caps what a single ladder can spend before anybody is dialled.
type Safety struct {
	// MaxCallsPerLadder is the most calls one ladder may place across all of
	// its rungs. A plan larger than this does not start, and says so: the
	// point is to make a rota mistake -- a rung that suddenly resolves to
	// forty people -- fail loudly at planning time rather than arrive as a
	// bill. Zero means no cap.
	MaxCallsPerLadder int `yaml:"maxCallsPerLadder"`
	// MaxLevel is the highest rung the ladder may climb, 0..4. Setting it to
	// 2 keeps an escalation away from the heads while a policy is being
	// trialled.
	//
	// A pointer, so that absent and zero are different things. LEVEL_0 is a
	// legitimate cap -- "first responders only" -- so it cannot double as the
	// "no cap" value, and a plain int would make the zero value of this whole
	// struct mean "truncate every ladder to one rung". That zero value is
	// what a deployment with no configuration file gets, and what every test
	// that builds an EngineConfig by hand gets, so it has to be the safe one.
	MaxLevel *int `yaml:"maxLevel"`
	// AllowedNumbers, when non-empty, is the only set of numbers that may be
	// dialled; every other recipient is dropped before the call is placed.
	//
	// This is what makes a real ladder safe to exercise against a real Twilio
	// account: the resolver still runs, the plan is still built from the real
	// rota, and only the numbers named here actually ring.
	AllowedNumbers []string `yaml:"allowedNumbers"`
}

// DefaultConfig is what the service does when no file is supplied: nothing.
//
// Disabled rather than permissive, because this file governs spending. A
// deployment that has not said what it wants dialled should dial nothing, and
// an operator reading the logs should see a refusal rather than a ladder built
// from defaults nobody chose.
func DefaultConfig() Config {
	return Config{
		Enabled: false,
		CRE:     LadderConfig{Channel: ChannelCall},
		SRE:     LadderConfig{Channel: ChannelCall},
	}
}

// LoadConfig reads and validates the configuration file.
//
// An unreadable or invalid file is an error, never a silent fall back to
// defaults. The caller disables both ladders and logs it. A file that exists
// but cannot be parsed is the case this protects hardest: it usually means
// somebody has just edited it, believes they have changed the behaviour, and
// would otherwise get the old behaviour with no indication.
func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig(), fmt.Errorf("escalation: read config %s: %w", path, err)
	}

	cfg := DefaultConfig()

	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	// An unknown key is an error, not something to ignore. A misspelled
	// `maxCallsPerLadder` that is quietly dropped leaves the operator certain
	// they have capped the spend when they have not.
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("escalation: parse config %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return DefaultConfig(), fmt.Errorf("escalation: invalid config %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if err := c.CRE.validate("cre"); err != nil {
		return err
	}
	return c.SRE.validate("sre")
}

func (l *LadderConfig) validate(name string) error {
	ch, err := ParseChannel(string(l.Channel))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	l.Channel = ch

	if m := l.Safety.MaxLevel; m != nil && (*m < int(Level0) || *m > int(Level4)) {
		return fmt.Errorf("%s: safety.maxLevel is %d; use %d..%d, or omit it for no cap",
			name, *m, Level0, Level4)
	}
	if l.Safety.MaxCallsPerLadder < 0 {
		return fmt.Errorf("%s: safety.maxCallsPerLadder is negative", name)
	}
	// A cap keyed by a shift name that does not exist would read as a cap and
	// do nothing -- the same failure KnownFields(true) exists to prevent, which
	// cannot see inside a map's keys.
	for k, v := range l.Teams.RotaMembersToCall {
		shift := Shift(strings.ToUpper(strings.TrimSpace(k)))
		if !shift.Known() {
			return fmt.Errorf("%s: teams.rotaMembersToCall names shift %q, which is not one of %s",
				name, k, strings.Join(KnownShiftNames(), ", "))
		}
		if v < 0 {
			return fmt.Errorf("%s: teams.rotaMembersToCall[%s] is negative; use 0 or omit it to call everyone on duty",
				name, k)
		}
	}

	// Only a table that was actually supplied is validated: an empty one means
	// "use the shipped rules", which are validated by their own test.
	if len(l.Rules) > 0 {
		if err := Validate(l.Rules); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// Ladder keys, so that looking a ladder's configuration up costs nothing in
// coupling. The SRE ladder's own Ladder type lives beside the SRE ladder; this
// file is read by both and should not need to know that type exists.
const (
	LadderKeyCRE = "cre"
	LadderKeySRE = "sre"
)

// For returns a ladder's configuration and whether it may run at all -- which
// takes both the master switch and the ladder's own. An unrecognised key
// yields the CRE configuration, since CRE is what an unclassified incident
// climbs.
func (c Config) For(key string) (LadderConfig, bool) {
	l := c.CRE
	if key == LadderKeySRE {
		l = c.SRE
	}
	return l, c.Enabled && l.Enabled
}

// Allows reports whether an incident matching these attributes should get a
// ladder, and when it should not, says why in words fit for a log line.
//
// team is the incident's assignment group and may be empty, which is what
// RequireKnownTeam exists to judge. shift is the derived shift code.
func (l LadderConfig) Allows(priority, team, shift string) (bool, string) {
	if l.Start.RequireKnownTeam && strings.TrimSpace(team) == "" {
		return false, "the incident names no team and trigger.requireKnownTeam is set"
	}
	// Both sides go through the same normalisation. The allowlist is written
	// the way the rules document spells it (S0..S4) while the incident carries
	// whatever the publisher sent (CRITICAL, P1, MODERATE), and comparing
	// those raw meant a correctly written allowlist matched nothing at all --
	// escalation silently off, with only an Info line per incident to say so.
	if !matchesPriority(l.Start.Priorities, priority) {
		return false, fmt.Sprintf("priority %s is not in trigger.priorities", priority)
	}
	if contains(l.Start.ExcludeTeams, team) {
		return false, fmt.Sprintf("team %s is in trigger.excludeTeams", team)
	}
	if !matches(l.Start.Teams, team) {
		return false, fmt.Sprintf("team %s is not in trigger.teams", team)
	}
	if !matches(l.Start.Shifts, shift) {
		return false, fmt.Sprintf("shift %s is not in trigger.shifts", shift)
	}
	return true, ""
}

// CapLevel is the highest rung this ladder may climb.
func (l LadderConfig) CapLevel() (Level, bool) {
	m := l.Safety.MaxLevel
	if m == nil || *m < int(Level0) || *m > int(Level4) {
		return Level4, false
	}
	return Level(*m), true
}

// Dialable reports whether a number may be called under this ladder's safety
// rules. An empty AllowedNumbers imposes no restriction.
func (l LadderConfig) Dialable(number string) bool {
	if len(l.Safety.AllowedNumbers) == 0 {
		return true
	}
	return contains(l.Safety.AllowedNumbers, number)
}

// matchesPriority is the allowlist test for priorities, comparing both sides
// in P-notation so S1, P1 and CRITICAL are one value.
func matchesPriority(allow []string, v string) bool {
	if len(allow) == 0 {
		return true
	}
	want := NormalisePriority(v)
	for _, item := range allow {
		if NormalisePriority(item) == want {
			return true
		}
	}
	return false
}

// matches is an allowlist test where empty means "no opinion". Comparison is
// case-insensitive and ignores surrounding space, because these values are
// typed by hand into a YAML file and copied out of ServiceNow.
func matches(allow []string, v string) bool {
	if len(allow) == 0 {
		return true
	}
	return contains(allow, v)
}

func contains(list []string, v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, item := range list {
		if strings.ToLower(strings.TrimSpace(item)) == v {
			return true
		}
	}
	return false
}
