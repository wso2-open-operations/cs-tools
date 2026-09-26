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
//
// Word lists this tool combines at runtime into synthetic people, companies,
// products, and ticket text. Every name here is a generic, non-attributable
// placeholder -- not drawn from any real person, company, or customer list --
// so the combinations this program builds are never mistaken for real data.
// Nothing generated from these lists is ever committed: only this source
// file is.
package main

var firstNames = []string{
	"Alex", "Jordan", "Taylor", "Morgan", "Casey", "Riley", "Avery", "Quinn",
	"Reese", "Sam", "Drew", "Jamie", "Skyler", "Rowan", "Emerson", "Hayden",
	"Parker", "Dakota", "Sage", "Finley", "Charlie", "Robin", "Blake", "Nico",
}

var lastNames = []string{
	"Rivera", "Chen", "Novak", "Patel", "Kim", "Okafor", "Larsen", "Haddad",
	"Tanaka", "Moreau", "Santos", "Berg", "Fischer", "Okonkwo", "Vance",
	"Reyes", "Olsen", "Marsh", "Ibarra", "Delgado", "Andersen", "Costa",
	"Whitfield", "Sorensen",
}

var companyAdjectives = []string{
	"Nova", "Summit", "Cascade", "Orbit", "Harbor", "Vertex", "Meridian",
	"Beacon", "Atlas", "Quartz", "Lumen", "Trellis", "Pioneer", "Horizon",
	"Anchor", "Crescent", "Granite", "Solstice", "Junction", "Compass",
}

var companyNouns = []string{
	"Systems", "Dynamics", "Networks", "Analytics", "Cloud", "Labs", "Works",
	"Holdings", "Solutions", "Technologies", "Industries", "Ventures",
	"Digital", "Robotics", "Logistics", "Platforms", "Innovations", "Group",
	"Partners", "Enterprises",
}

// emailWordSuffixes stand in for the "+<random>" disambiguator described in
// the task brief -- combined with a random number at generation time so two
// runs never collide on the same address even before the marker-table gate
// is checked (see main.go).
var emailProviders = []string{"example.com", "example.org", "example.net"}

// productCatalog is a small, fixed set of generic, non-trademarked product
// names -- not real WSO2 (or any vendor's) product names -- paired with a
// product_unit_enum value each (see entity-service migrations/000010).
var productCatalog = []struct {
	name         string
	unit         string // product_unit_enum
	businessUnit string // product_business_unit_enum
}{
	{"API Gateway", "APIM", "INTEGRATION_SOFTWARE"},
	{"Identity Hub", "IAM", "IAM"},
	{"Integration Engine", "INTEGRATION", "INTEGRATION_SOFTWARE"},
	{"Insight Analytics", "ANALYTICS", "INTEGRATION_SOFTWARE"},
	{"Cloud Runtime", "CHOREO", "INTEGRATION_CLOUD"},
	{"Device Mesh", "IOT", "INTEGRATION_SOFTWARE"},
}

var productVersionNumbers = []string{"3.0.0", "3.1.0", "4.0.0", "4.1.2", "4.2.0", "5.0.0"}

var caseSubjectVerbs = []string{
	"Intermittent", "Recurring", "Sudden", "Unexpected", "Persistent", "Sporadic",
}

var caseSubjectProblems = []string{
	"connection timeouts", "authentication failures", "latency spikes",
	"memory pressure", "throttling errors", "configuration drift",
	"certificate expiry warnings", "queue backlog", "startup failures",
	"data sync mismatches", "rate-limit rejections", "log flooding",
}

var caseSubjectEnvs = []string{
	"production", "staging", "the primary cluster", "the DR environment",
	"the customer's on-prem deployment", "the managed cloud tenant",
}

var caseDescriptionTemplates = []string{
	"Customer reports %s affecting %s. Needs investigation and a root-cause update.",
	"Observed %s in %s after the last scheduled maintenance window.",
	"Support case opened for %s impacting %s; customer has requested a status update.",
	"Monitoring flagged %s in %s; customer confirmed impact on their side.",
}

var srSubjects = []string{
	"Request to increase rate limit quota",
	"Assistance needed with SSO configuration",
	"Request for a new API key",
	"Guidance requested on upgrade path",
	"Request to onboard an additional environment",
	"Question about supported deployment topologies",
	"Request for a configuration review",
	"Assistance needed with log export setup",
}

var crSubjects = []string{
	"Scheduled certificate rotation",
	"Planned capacity increase",
	"Apply security patch to runtime",
	"Migrate deployment to new region",
	"Upgrade to latest supported version",
	"Network firewall rule update",
}

var incidentSubjects = []string{
	"Service interruption reported by monitoring",
	"Elevated error rate on core API",
	"Partial outage affecting a subset of tenants",
	"Database connection pool exhaustion",
	"Unexpected node restart in the cluster",
}

var engagementSubjects = []string{
	"Migration engagement kickoff",
	"Onboarding engagement for new deployment",
	"Consultancy engagement on integration architecture",
	"Follow-up engagement after go-live",
}

var problemSubjects = []string{
	"Recurring cluster restarts under investigation",
	"Root cause analysis for repeated timeouts",
	"Known error: intermittent queue backlog",
	"Chronic memory growth under sustained load",
}

var commentTemplates = []string{
	"Investigating further, will update shortly.",
	"Reproduced the issue in our test environment.",
	"Applied a configuration change; monitoring for recurrence.",
	"Waiting on additional logs from the customer.",
	"Root cause identified; preparing a fix.",
	"Customer confirmed the workaround resolves the immediate impact.",
	"Escalated internally for a second opinion.",
	"Verified the fix in staging before rollout.",
	"No further reports from the customer after the last change.",
	"Following up to confirm this can be closed.",
}

var workNoteTemplates = []string{
	"Internal note: paged the on-call engineer.",
	"Internal note: correlated with a known upstream advisory.",
	"Internal note: coordinating with the platform team.",
	"Internal note: change request linked for the underlying fix.",
}

var escalationReasons = []string{
	"No update from support in over 24 hours on a S1 case.",
	"Customer requested management visibility due to business impact.",
	"Repeated regression after two prior fix attempts.",
	"SLA breach imminent without further action.",
}

var tagNames = []string{
	"regression", "upgrade", "security", "performance", "documentation",
	"billing", "networking", "data-migration",
}

var timeCardComments = []string{
	"Investigated logs and reproduced the reported behaviour.",
	"Reviewed configuration with the customer over a call.",
	"Applied and verified the suggested workaround.",
	"Prepared root-cause summary for the customer.",
	"Paired with another engineer to narrow down the cause.",
}
