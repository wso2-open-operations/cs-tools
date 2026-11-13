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

import MultiSelectField from "@components/MultiSelectField";
import {
  dateAdvancedField,
  multiAdvancedField,
  type OperationsAdvancedField,
} from "@features/csm-operations/utils/operationsAdvancedFields";
import {
  PROJECT_CLOSURE_STATES,
  PROJECT_DATE_FILTER_KEYS,
  PROJECT_DATE_FILTER_LABELS,
  type ProjectFilters,
} from "@features/csm-projects/utils/projectFilters";

/** The vocabulary `POST /projects/search` and a dashboard widget's `project`
 *  query use for `onboardingStatus` (`OnHold`, not `On-Hold`); "Not
 *  Applicable" is no state to filter *for*. */
export const ONBOARDING_STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: "In-Progress", label: "In Progress" },
  { value: "Completed", label: "Completed" },
  { value: "Not-Started", label: "Not Started" },
  { value: "Expired", label: "Expired" },
  { value: "Cancelled", label: "Cancelled" },
  { value: "OnHold", label: "On Hold" },
];

const CLOSURE_STATE_OPTIONS = PROJECT_CLOSURE_STATES.map((s) => ({ value: s, label: s }));

/** The Advanced builder's fields: project state, onboarding status, and one
 *  on-or-after / on-or-before row pair per date column. Each reads and writes
 *  the same state the URL and the request use. Dates may be in the future (a
 *  planned go-live), so none is capped at today. */
export function projectAdvancedFields(): OperationsAdvancedField<ProjectFilters>[] {
  return [
    multiAdvancedField<ProjectFilters>({
      key: "closureStates",
      label: "Project state",
      prop: "closureStates",
      render: ({ id, values, onChange }) => (
        <MultiSelectField id={id} label="Value(s)" values={values} options={CLOSURE_STATE_OPTIONS} onChange={onChange} />
      ),
    }),
    multiAdvancedField<ProjectFilters>({
      key: "onboardingStatus",
      label: "Onboarding status",
      prop: "onboardingStatus",
      render: ({ id, values, onChange }) => (
        <MultiSelectField id={id} label="Value(s)" values={values} options={ONBOARDING_STATUS_OPTIONS} onChange={onChange} />
      ),
    }),
    ...PROJECT_DATE_FILTER_KEYS.map((k) =>
      dateAdvancedField<ProjectFilters>({
        key: k,
        label: PROJECT_DATE_FILTER_LABELS[k],
        from: `${k}From`,
        to: `${k}To`,
        capAtToday: false,
      }),
    ),
  ];
}
