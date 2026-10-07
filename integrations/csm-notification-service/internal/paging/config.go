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

package paging

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

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

	// Which ladders a case or an incident climbs is deliberately NOT here:
	// it is DefaultRouting, in code. Those rules (case S0 -> CRE + SRE, S1-S4
	// -> CRE, SRE incident -> SRE) are the Case Paging design, and changing
	// them is a code change with a review, not a file edit on a running
	// deployment (decided 2026-10-07). A file that still carries a routing
	// section is refused: an unknown key.
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
	// row here rather than waiting for a release. CRE only: the SRE ladder
	// routes by on-call tier, which has one path.
	Rules []Rule `yaml:"rules"`
	// Timing is the SRE ladder's clock. SRE only: the CRE ladder's clock is
	// section 7.0's per-priority table.
	Timing SRETiming `yaml:"timing"`

	Chat Chat `yaml:"chat"`

	// Phones is where a recipient's phone number comes from when the rung's
	// resolver does not supply one (the Team Schedule never does).
	Phones Phones `yaml:"phones"`
}

// Phones selects the source of recipients' phone numbers.
//
//	profile  the number each person set on their own CSM Portal profile, read
//	         from Asgardeo (the default)
//	none     only numbers named in this file; everyone else is NO_NUMBER
type Phones struct {
	Source string `yaml:"source"`
}

// Phone sources.
const (
	PhoneSourceProfile = "profile"
	PhoneSourceNone    = "none"
)

// PhoneSource is the configured source, defaulting to the profile.
func (l LadderConfig) PhoneSource() string {
	if s := strings.ToLower(strings.TrimSpace(l.Phones.Source)); s != "" {
		return s
	}
	return PhoneSourceProfile
}

// SRETiming is the SRE ladder's clock: one call per rung, a fixed gap between
// rungs, the same for every priority.
type SRETiming struct {
	// InitialWait is how long after the incident L1 support is called.
	// Absent means zero -- the diagram calls L1 the moment it is created.
	InitialWait Duration `yaml:"initialWait"`
	// Interval is the gap between two rungs. Absent means five minutes.
	Interval Duration `yaml:"interval"`
	// IncludeL4 adds the fourth rung, L4 support. NOT CONFIRMED: the rota's
	// tiers stop at L3, and L4 resolves to the SRE team's lead on an
	// assumption nobody has signed off.
	IncludeL4 bool `yaml:"includeL4"`
}

// Policy turns the timing into the policy shape the planner reads.
func (t SRETiming) Policy() PriorityPolicy {
	interval := time.Duration(t.Interval)
	if interval <= 0 {
		interval = sreStep
	}
	rung := LevelPolicy{NotificationCount: 1, NotificationInterval: interval}
	p := PriorityPolicy{InitialWait: time.Duration(t.InitialWait)}
	p.Levels[Level0] = rung
	p.Levels[Level1] = rung
	p.Levels[Level2] = rung
	if t.IncludeL4 {
		p.Levels[Level3] = rung
	}
	return p
}

// Duration is a time.Duration written the way a person writes one in YAML:
// "5m", "30s".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(strings.TrimSpace(n.Value))
	if err != nil {
		return fmt.Errorf("line %d: %q is not a duration; write it like 5m or 30s", n.Line, n.Value)
	}
	*d = Duration(v)
	return nil
}

// Chat is where a ladder's rung cards are posted.
type Chat struct {
	// Audience is the GOOGLE_CHAT_SPACES key every rung card goes to. Empty
	// falls back to INCIDENT_ESCALATION_CHAT_AUDIENCE, then to
	// "Incident Monitor" -- the room every other incident alert in this
	// service posts to.
	//
	// Configured rather than derived. The card used to route by the
	// incident's product, which no audience is ever named after, so on a
	// deployment with Chat set up correctly every card was silently dropped.
	// An audience missing from GOOGLE_CHAT_SPACES is now a recorded failure,
	// NO_CHAT_SPACE, not a quiet success.
	Audience string `yaml:"audience"`

	// WebhookURLEnv is the NAME of an environment variable holding this
	// ladder's own Google Chat webhook URL -- never the URL itself. When set,
	// rung cards go straight to that space and GOOGLE_CHAT_SPACES is not
	// consulted; Audience then only labels the room in logs.
	//
	// The URL carries the space's key and token, so anyone holding it can post
	// to the room, and this file is committed to a public repository. Naming
	// the variable keeps the choice of room in configuration -- changing it is
	// still a file edit, not a release -- while the secret stays in .env
	// locally and in a secret store when deployed. A value that is not a
	// variable name is refused at load, so a URL pasted here by mistake fails
	// before it can be committed.
	//
	// Named but empty is NOT a fallback to GOOGLE_CHAT_SPACES: every rung is
	// recorded NO_CHAT_SPACE and startup says which variable is missing.
	// Quietly posting to some other room is worse than visibly posting nowhere.
	WebhookURLEnv string `yaml:"webhookUrlEnv"`
}

// envVarName is what chat.webhookUrlEnv must look like.
var envVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
	// CallHeadsWithoutLowerTiers lets the heads' tiers (LEVEL_3 CRE head,
	// LEVEL_4 CS head) ring even when nobody on LEVEL_0..LEVEL_2 can be
	// called. Off by default, which holds the heads' calls in that case.
	//
	// The ladder exists so the people closest to an incident answer first. A
	// plan in which no first responder, team lead or lead has a number is not
	// an incident nobody answered -- it is missing phone numbers -- and
	// paging a director and a VP for it, with nobody below them having been
	// given a chance, is the wrong answer to a data problem. Held calls are
	// logged, not written to the work note. With channel "both" the heads'
	// chat cards still post; only the calls are held.
	CallHeadsWithoutLowerTiers bool `yaml:"callHeadsWithoutLowerTiers"`
	// CallWithoutVerifiedLeads lets a ladder place calls before every lead in
	// its ABT lead pool has a number to call. Off by default, which holds all
	// of a ladder's calls until the pool is complete.
	//
	// The lead tiers (LEVEL_1, LEVEL_2) are what stand between the first
	// responders and the heads. With a lead missing a number those tiers are
	// skipped as NO_NUMBER, and an unanswered incident -- however low its
	// priority -- goes from LEVEL_0 straight to a director and a VP. So calls
	// are switched on by the data being ready, not by a date: once every lead
	// has set a mobile number on their CSM Portal profile, calls start on the
	// next incident. A held ladder is logged (naming the leads without a
	// number), never written to the work note; with channel "both" its chat
	// cards still post and only the calls are held.
	CallWithoutVerifiedLeads bool `yaml:"callWithoutVerifiedLeads"`
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
	if err := c.SRE.validate("sre"); err != nil {
		return err
	}
	// A team is an ABT or an SRE team, never both. Listed as both, its
	// incidents would climb the SRE ladder while its lead was still being
	// called on every CRE ladder's all_team_leads rung.
	for _, k := range c.SRE.Teams.ABTs {
		if contains(c.CRE.Teams.ABTs, k) {
			return fmt.Errorf("team %q is in both cre.teams.abts and sre.teams.abts", k)
		}
	}
	return nil
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
	// Never echo the value: if it is not a variable name it is most likely the
	// webhook URL itself, which is a secret.
	if v := strings.TrimSpace(l.Chat.WebhookURLEnv); v != "" && !envVarName.MatchString(v) {
		return fmt.Errorf("%s: chat.webhookUrlEnv must be the NAME of an environment variable "+
			"that holds the webhook URL, not the URL itself -- this file is committed", name)
	}

	if src := l.PhoneSource(); src != PhoneSourceProfile && src != PhoneSourceNone {
		return fmt.Errorf("%s: phones.source is %q; use %q or %q", name, l.Phones.Source,
			PhoneSourceProfile, PhoneSourceNone)
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

	// Each ladder's own knobs are refused on the other, rather than accepted
	// and ignored: an SRE clock written under cre: would otherwise look like
	// it had changed something.
	if name == LadderKeySRE {
		if len(l.Rules) > 0 {
			return fmt.Errorf("sre: rules is a CRE setting; the SRE ladder routes by on-call tier")
		}
		if l.Timing.Interval < 0 || l.Timing.InitialWait < 0 {
			return fmt.Errorf("sre: timing durations must not be negative")
		}
	} else {
		if l.Timing != (SRETiming{}) {
			return fmt.Errorf("%s: timing is an SRE setting; the CRE clock is the per-priority policy", name)
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
