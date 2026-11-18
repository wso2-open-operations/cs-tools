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


import { useMemo, type JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import { useAccountRegions } from "@features/csm-projects/api/useAccountRegions";

interface ProjectRegionMultiSelectProps {
  id: string;
  values: string[];
  onChange: (next: string[]) => void;
}

/**
 * The projects list's Region value picker: the account regions
 * ({@link useAccountRegions}), plus any chosen region the list does not hold
 * (one from a link or a dashboard click-through), so a selection never
 * silently disappears from the field.
 */
export default function ProjectRegionMultiSelect({
  id,
  values,
  onChange,
}: ProjectRegionMultiSelectProps): JSX.Element {
  const { data: regions, isLoading, isError } = useAccountRegions(true);
  const options = useMemo(() => {
    const all = [...(regions ?? [])];
    const known = new Set(all.map((r) => r.toLowerCase()));
    for (const v of values) {
      if (!known.has(v.toLowerCase())) all.push(v);
    }
    return all.map((r) => ({ value: r, label: r }));
  }, [regions, values]);

  const unavailable = !isLoading && options.length === 0;
  return (
    <MultiSelectField
      id={id}
      label="Value(s)"
      values={values}
      options={options}
      onChange={onChange}
      disabled={isLoading || unavailable}
      disabledTooltip={
        isLoading
          ? "Loading regions…"
          : isError
            ? "Regions couldn't be loaded."
            : "No account has a region recorded."
      }
    />
  );
}
