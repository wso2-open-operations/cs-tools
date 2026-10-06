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

// Ported from SupportPortalLite's src/data/utils/types.ts — the
// GET /abt-team-schedule response shape. Kept local to this domain folder
// rather than a shared types file, since this port is split across several
// independent agents working in parallel.

export interface ABTTeamScheduleData {
  list: ABTTeamScheduleList[];
  metadata: ABTTeamScheduleMetadata[];
  snURL: string;
}

export interface ABTTeamScheduleList {
  label: string;
  members: ABTTeamScheduleMember[];
}

export interface ABTTeamScheduleMember {
  name: string;
  roles: ABTTeamScheduleMemberRole[];
  schedule: { [date: string]: ABTTeamScheduleMemberSchedule[] };
}

export interface ABTTeamScheduleMemberRole {
  name: string;
  label: string;
}

export interface ABTTeamScheduleMemberSchedule {
  name: string;
  label: string;
}

export interface ABTTeamScheduleMetadata {
  teams: ABTTeamScheduleMetadataTeam[];
  eventTypes: ABTTeamScheduleMetadataEventType[];
}

export interface ABTTeamScheduleMetadataTeam {
  label: string;
  id: string;
  link: string;
}

export interface ABTTeamScheduleMetadataEventType {
  name: string;
  label: string;
}
