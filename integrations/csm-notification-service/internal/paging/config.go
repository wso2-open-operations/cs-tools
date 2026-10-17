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

	// SME is the Special Ops page the SRE engine places when an incident is
	// escalated to a Special Ops team. Absent is off.
	SME SMEConfig `yaml:"sme"`

	// TestCall governs the "Test call" a lead or admin places to verify a
	// person's paging-only number (testcall.go). Absent is on, any number.
	TestCall TestCallConfig `yaml:"testCall"`

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
	// OnLeave is what a rung does with someone who holds it but is on leave
	// that day: skip (the default) leaves them out, so the rung goes to
	// whoever else holds it or the ladder climbs past; call rings them
	// anyway, after anyone available on the same tier.
	OnLeave OnLeave `yaml:"onLeave"`
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
	// TeamChats gives SRE teams Google Chat spaces of their own, by team key
	// (apollo, artemis, iaas), each the same shape as chat: a team's rung
	// cards and closing message go there, and an incident whose team is not
	// listed (or has none) uses chat above. SRE only.
	TeamChats map[string]Chat `yaml:"teamChats"`
	// TeamlessChats is the teamChats keys an incident with no SRE team of its
	// own posts to, every one of them: a default group (a service mapped to
	// no SRE team), a customer case at S0, a monitoring alert with no team.
	// [apollo, artemis] notifies both SaaS teams. Empty, such an incident
	// posts to chat above, as before. Its closing message follows. SRE only.
	TeamlessChats []string `yaml:"teamlessChats"`
	// TeamChannels is how each SRE team's alerts go out (chat, call, both,
	// log), by team key, set on the Team Schedule's Alert Settings tab (Team
	// routing) and never from this file. A team's channel can only narrow the
	// ladder's channel above (narrowChannel); a team not listed uses it as it
	// is. SRE only.
	TeamChannels map[string]Channel `yaml:"-"`
	// UnansweredChat posts a closing message when the ladder has called every
	// rung and nobody acknowledged the incident, to the team's own space
	// (teamChats), else chat. It is posted whatever channel says, so a
	// call-only ladder still tells the room the calls have ended. SRE only.
	// An incident with an INCIDENT_CHAT_SPACES space gets it there whether this is on or not.
	UnansweredChat bool `yaml:"unansweredChat"`
	// SMEHandoffChat posts one message to the incident's SRE team space
	// (teamChats, else chat) when it is escalated to an SME team: who
	// escalated it, that SRE paging stopped, and that the SMEs are being
	// paged. Posted whatever channel says. SRE only.
	// An incident with an INCIDENT_CHAT_SPACES space gets it there whether this is on or not.
	SMEHandoffChat bool `yaml:"smeHandoffChat"`
	// AckChat posts one reply in a ladder's thread when the ladder stops for
	// any reason but a hand-off to the SMEs (which posts its own): who
	// acknowledged it, or that the case closed or its severity changed, and
	// how many calls were cancelled. It goes where the rung cards went, and
	// only once a rung has gone out, so a room is never told about a chain it
	// never saw. The SRE ladder's setting covers its SME ladders too. Defaults
	// to on; a pointer so a deployment can turn it off.
	AckChat *bool `yaml:"ackChat"`

	// Phones is where a recipient's phone number comes from when the rung's
	// resolver does not supply one (the Team Schedule never does).
	Phones Phones `yaml:"phones"`

	// Email sends one email per rung, L1 to L4, alongside the calls and
	// cards. CRE only; absent is off.
	Email RungEmail `yaml:"email"`
}

// RungEmail is the Case Paging rung email (rung_email.go). LEVEL_0 gets none.
//
//	L1    To the case's ABT team lead, Cc the case's ABT team list
//	L2    To every ABT team lead, Cc every ABT team list
//	L3    To the CRE head, Cc creTeamList
//	L4    To the CS head,  Cc csTeamList
//
// At night (21:00-06:00 IST) L1 and L2 also go To the Americas team leads
// (L1) and the America Team lead (L2), and also Cc the Americas team list.
type RungEmail struct {
	Enabled bool `yaml:"enabled"`
	// TeamLists maps a team key to its mailing list: the ABT teams
	// (teams.abts) and the Americas team (teams.americas). A key that is
	// neither is refused.
	TeamLists map[string]string `yaml:"teamLists"`
	// CRETeamList and CSTeamList are Cc'd on L3 and L4.
	CRETeamList string `yaml:"creTeamList"`
	CSTeamList  string `yaml:"csTeamList"`
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

// SMEConfig is the Special Ops (Subject Matter Specialist) page: when a SaaS
// SRE incident moves into a Special Ops group (incident.special_ops_alert), the
// SaaS SRE chain stops and ONE call goes to the SME on duty for that team's
// current Day/Night window. It is not a ladder -- no rungs, no clock -- so it has none
// of a ladder's trigger, timing or caps; only where the page goes, and which
// numbers it may ring. See sme.go.
type SMEConfig struct {
	// Enabled turns the page on. Absent is off: a deployment that has not
	// opted in ignores the alert, as it did before the page existed.
	Enabled bool `yaml:"enabled"`
	// Channel is log, chat, call or both, as for a ladder. Empty is call.
	Channel Channel `yaml:"channel"`
	// Chat is the SME page's own Google Chat space, the same shape as a
	// ladder's chat section.
	Chat Chat `yaml:"chat"`
	// TeamChats gives SME teams Google Chat spaces of their own, by SME team
	// key (the rota team key: asgardeo, b-central, ...), each the same shape
	// as chat. Empty, every team's cards go to chat above, as before. Set, a
	// listed team's cards go to its own space and a team not listed gets no
	// card; its calls still go out when the channel includes calls.
	TeamChats map[string]Chat `yaml:"teamChats"`
	// UnansweredChat posts a closing message when the SME ladder has called
	// L1 to L3 and nobody was assigned: to the SME team's own space when
	// teamChats lists it, to chat when teamChats is unset, and nowhere for a
	// team teamChats leaves out. Posted whatever channel says.
	UnansweredChat bool `yaml:"unansweredChat"`
	// Teams maps a Special Ops team key (incident.special_ops_alert's
	// teamKey) to the rota team key of the SME team to page -- for an alert
	// that does not name the SME team itself.
	Teams map[string]string `yaml:"teams"`
	// Safety is what the SME ladder may ring.
	Safety SMESafety `yaml:"safety"`
	// OnLeave is the ladders' onLeave for the SME ladder: skip (the default)
	// or call.
	OnLeave OnLeave `yaml:"onLeave"`
	// Timing is the SME ladder's clock, the same shape and defaults as
	// sre.timing: L1 at once, L2 after one interval, L3 after two (five
	// minutes each by default). includeL4 is ignored: SME shifts stop at L3.
	Timing SRETiming `yaml:"timing"`
}

// OnLeave is what a ladder does with someone on leave who holds a rung.
type OnLeave string

const (
	// OnLeaveSkip leaves them out (the default, also when absent).
	OnLeaveSkip OnLeave = "skip"
	// OnLeaveCall rings them anyway, after anyone available on the same tier.
	OnLeaveCall OnLeave = "call"
)

// Calls reports whether someone on leave is rung.
func (o OnLeave) Calls() bool { return o == OnLeaveCall }

func (o *OnLeave) validate(name string) error {
	v := OnLeave(strings.ToLower(strings.TrimSpace(string(*o))))
	switch v {
	case "", OnLeaveSkip:
		*o = OnLeaveSkip
	case OnLeaveCall:
		*o = OnLeaveCall
	default:
		return fmt.Errorf("%s: onLeave is %q; use skip or call", name, string(*o))
	}
	return nil
}

// Policy is the SME ladder's clock as the planner reads it: L1-L3 only.
func (s SMEConfig) Policy() PriorityPolicy {
	t := s.Timing
	t.IncludeL4 = false
	return t.Policy()
}

// SMESafety is what the SME ladder may ring.
type SMESafety struct {
	// AllowedNumbers, when non-empty, is the only set of numbers the page may
	// call, as a ladder's safety.allowedNumbers.
	AllowedNumbers []string `yaml:"allowedNumbers"`
}

// Dialable reports whether the SME page may call this number.
func (s SMEConfig) Dialable(number string) bool {
	return len(s.Safety.AllowedNumbers) == 0 || contains(s.Safety.AllowedNumbers, number)
}

// TeamFor is the SME team key sme.teams maps a Special Ops team to, or "".
func (s SMEConfig) TeamFor(specialOpsTeam string) string {
	want := teamKeyFor(specialOpsTeam)
	if want == "" {
		return ""
	}
	for k, v := range s.Teams {
		if teamKeyFor(k) == want {
			return teamKeyFor(v)
		}
	}
	return ""
}

func (s *SMEConfig) validate() error {
	ch, err := ParseChannel(string(s.Channel))
	if err != nil {
		return fmt.Errorf("sme: %w", err)
	}
	s.Channel = ch
	if err := s.OnLeave.validate("sme"); err != nil {
		return err
	}
	if v := strings.TrimSpace(s.Chat.WebhookURLEnv); v != "" && !envVarName.MatchString(v) {
		return fmt.Errorf("sme: chat.webhookUrlEnv must be the NAME of an environment variable " +
			"that holds the webhook URL, not the URL itself -- this file is committed")
	}
	seen := map[string]string{}
	for k, c := range s.TeamChats {
		key := teamKeyFor(k)
		if key == "" {
			return fmt.Errorf("sme: teamChats has an entry with no SME team key")
		}
		if prev, dup := seen[key]; dup {
			return fmt.Errorf("sme: teamChats lists SME team %q twice (%q and %q)", key, prev, k)
		}
		seen[key] = k
		if v := strings.TrimSpace(c.WebhookURLEnv); v != "" && !envVarName.MatchString(v) {
			return fmt.Errorf("sme: teamChats.%s.webhookUrlEnv must be the NAME of an environment variable "+
				"that holds the webhook URL, not the URL itself -- this file is committed", k)
		}
	}
	for k, v := range s.Teams {
		if teamKeyFor(k) == "" || teamKeyFor(v) == "" {
			return fmt.Errorf("sme: teams maps %q to %q; both the escalation team and the SME team key are required", k, v)
		}
	}
	return nil
}

// TestCallConfig is what a paging-number test call may do.
type TestCallConfig struct {
	// Enabled is a pointer so absent and false differ: absent is on. A test
	// call is one short call a person asked for, to their own number.
	Enabled *bool `yaml:"enabled"`
	// AllowedNumbers, when non-empty, is the only set of numbers a test call
	// may ring; anything else is reported "failed".
	AllowedNumbers []string `yaml:"allowedNumbers"`
}

// On reports whether test calls are placed at all.
func (t TestCallConfig) On() bool { return t.Enabled == nil || *t.Enabled }

// Dialable reports whether a test call may ring this number.
func (t TestCallConfig) Dialable(number string) bool {
	return len(t.AllowedNumbers) == 0 || contains(t.AllowedNumbers, number)
}

// sreRotaCode is what sre.teams.defaultRota must look like: an SRE rota code.
var sreRotaCode = regexp.MustCompile(`^SRE_[A-Z0-9_]+$`)

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

// AckChatOn reports whether a stopped ladder replies in its thread (AckChat).
func (l LadderConfig) AckChatOn() bool {
	return l.AckChat == nil || *l.AckChat
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
	if err := c.SME.validate(); err != nil {
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
	if err := l.OnLeave.validate(name); err != nil {
		return err
	}

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
		if l.Email.Enabled || len(l.Email.TeamLists) > 0 || l.Email.CRETeamList != "" || l.Email.CSTeamList != "" {
			return fmt.Errorf("sre: email is a CRE setting; the SRE ladder sends no rung email")
		}
		if l.Timing.Interval < 0 || l.Timing.InitialWait < 0 {
			return fmt.Errorf("sre: timing durations must not be negative")
		}
		if v := strings.TrimSpace(l.Teams.DefaultRota); v != "" && !sreRotaCode.MatchString(strings.ToUpper(v)) {
			return fmt.Errorf("sre: teams.defaultRota is %q; use an SRE rota code such as %s or %s", v, RotaSRESaaS, RotaSREIaaS)
		}
		listed := map[string]bool{}
		for k := range l.TeamChats {
			listed[teamKeyFor(k)] = true
		}
		named := map[string]bool{}
		for _, k := range l.TeamlessChats {
			key := teamKeyFor(k)
			if !listed[key] {
				return fmt.Errorf("sre: teamlessChats names %q, which has no space in teamChats", k)
			}
			if named[key] {
				return fmt.Errorf("sre: teamlessChats names %q twice", k)
			}
			named[key] = true
		}
		seen := map[string]string{}
		for k, c := range l.TeamChats {
			key := teamKeyFor(k)
			if key == "" {
				return fmt.Errorf("sre: teamChats has an entry with no team key")
			}
			if prev, dup := seen[key]; dup {
				return fmt.Errorf("sre: teamChats lists team %q twice (%q and %q)", key, prev, k)
			}
			seen[key] = k
			if v := strings.TrimSpace(c.WebhookURLEnv); v != "" && !envVarName.MatchString(v) {
				return fmt.Errorf("sre: teamChats.%s.webhookUrlEnv must be the NAME of an environment variable "+
					"that holds the webhook URL, not the URL itself -- this file is committed", k)
			}
		}
		for _, g := range l.Teams.DefaultGroups {
			if teamKeyFor(g) == "" {
				return fmt.Errorf("sre: teams.defaultGroups has an empty group name")
			}
			for _, t := range l.Teams.ABTs {
				if teamKeyFor(t) == teamKeyFor(g) {
					return fmt.Errorf("sre: %q is in both teams.abts and teams.defaultGroups; a group is an SRE team or a default group, not both", g)
				}
			}
		}
	} else {
		if l.Timing != (SRETiming{}) {
			return fmt.Errorf("%s: timing is an SRE setting; the CRE clock is the per-priority policy", name)
		}
		if strings.TrimSpace(l.Teams.DefaultRota) != "" {
			return fmt.Errorf("%s: teams.defaultRota is an SRE setting", name)
		}
		if len(l.Teams.DefaultGroups) > 0 {
			return fmt.Errorf("%s: teams.defaultGroups is an SRE setting", name)
		}
		if len(l.TeamChats) > 0 || l.UnansweredChat || l.SMEHandoffChat || len(l.TeamlessChats) > 0 {
			return fmt.Errorf("%s: teamChats, teamlessChats, unansweredChat and smeHandoffChat are SRE settings", name)
		}
		if err := l.Email.validate(name, l.Teams); err != nil {
			return err
		}
	}
	return nil
}

// oneAddress reports whether s is a single email address.
func oneAddress(s string) bool {
	a := strings.TrimSpace(s)
	return a != "" && !strings.ContainsAny(a, " ,;") && strings.Count(a, "@") == 1
}

// validate checks the rung email's lists. Never echoes an address.
func (m RungEmail) validate(name string, teams TeamKeys) error {
	for key, v := range map[string]string{"creTeamList": m.CRETeamList, "csTeamList": m.CSTeamList} {
		if strings.TrimSpace(v) != "" && !oneAddress(v) {
			return fmt.Errorf("%s: email.%s is not a single email address", name, key)
		}
	}
	for k, addr := range m.TeamLists {
		key := teamKeyFor(k)
		if key == "" {
			return fmt.Errorf("%s: email.teamLists has an entry with no team key", name)
		}
		if !oneAddress(addr) {
			return fmt.Errorf("%s: email.teamLists.%s is not a single email address", name, k)
		}
		// A key naming no team would read as a Cc and send nothing. Only
		// checked against a list that was written down: with teams.abts empty
		// the ABTs come from their type, and any key may be one of them.
		if len(teams.ABTs) > 0 && !contains(teams.ABTs, key) && key != teamKeyFor(teams.Americas) {
			return fmt.Errorf("%s: email.teamLists names team %q, which is neither in teams.abts nor teams.americas", name, k)
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
