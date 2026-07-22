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

import { Chip, Typography } from "@wso2/oxygen-ui";
import { Link as LinkIcon } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { BeEntityRef } from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";

/**
 * Renders a `BeEntityRef` (`{id, name}`) as a clickable chip navigating to
 * `${routeBase}/${value.id}` when the caller passes `routeBase` — only do so
 * when the field's target record type is independently known (e.g. a change
 * request's `case` is always a Case, an incident's `changeRequest` is always
 * a Change Request). When a reference field's target type can't be
 * determined safely (an ambiguous/generic link field), omit `routeBase` and
 * this renders as plain text instead of guessing at a possibly-wrong link.
 */
export default function EntityRefLink({
  value,
  routeBase,
}: {
  value?: BeEntityRef | null;
  routeBase?: string;
}): JSX.Element {
  const navigate = useNavTransition();

  if (!value) return <Typography variant="body2">—</Typography>;

  const label = value.name || value.id;

  if (!routeBase) return <Typography variant="body2">{label}</Typography>;

  return (
    <Chip
      size="small"
      variant="outlined"
      clickable
      icon={<LinkIcon size={14} />}
      label={label}
      onClick={() => navigate(`${routeBase}/${value.id}`)}
    />
  );
}
