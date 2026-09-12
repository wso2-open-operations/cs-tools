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

import { Box, Chip, Typography } from "@wso2/oxygen-ui";
import { Check } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import {
  CHANGE_REQUEST_FORWARD_STATES,
  changeRequestStateLabel,
  isChangeRequestOffRampState,
} from "@features/csm-operations/utils/changeRequests";

const NODE_SIZE = 22;

/**
 * One step marker: a filled circle for done/current, an outlined circle for
 * not-yet-reached. The current step additionally gets a soft halo (a larger,
 * low-opacity ring behind it) so it reads as "you are here" at a glance
 * rather than just "a slightly different colour than its neighbours".
 *
 * Current uses `info` rather than `primary`: `primary` is this app's brand
 * accent, already used everywhere (buttons, the active tab underline, links)
 * — reusing it here would blend the stepper into that noise instead of
 * standing out as its own "you are here" signal. `info` reads as
 * in-progress/informational and doesn't compete with any nearby primary CTA.
 */
function StepNode({ done, current }: { done: boolean; current: boolean }): JSX.Element {
  return (
    <Box sx={{ position: "relative", display: "flex", alignItems: "center", justifyContent: "center" }}>
      {current && (
        <Box
          aria-hidden
          sx={{
            position: "absolute",
            width: NODE_SIZE + 12,
            height: NODE_SIZE + 12,
            borderRadius: "50%",
            bgcolor: "info.main",
            opacity: 0.15,
          }}
        />
      )}
      <Box
        sx={{
          position: "relative",
          width: NODE_SIZE,
          height: NODE_SIZE,
          borderRadius: "50%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          flexShrink: 0,
          transition: "background-color 0.15s, border-color 0.15s",
          ...(done
            ? { bgcolor: "success.main", color: "success.contrastText" }
            : current
              ? { bgcolor: "info.main", color: "info.contrastText" }
              : {
                  bgcolor: "transparent",
                  border: "2px solid",
                  borderColor: "divider",
                }),
        }}
      >
        {done ? (
          <Check size={13} strokeWidth={3} />
        ) : current ? (
          <Box sx={{ width: 7, height: 7, borderRadius: "50%", bgcolor: "info.contrastText" }} />
        ) : null}
      </Box>
    </Box>
  );
}

/**
 * Horizontal lifecycle indicator for a change request's 9-state forward path
 * (`CHANGE_REQUEST_FORWARD_STATES`): a connected line of step markers — solid
 * and checked for every state already passed, a highlighted "current
 * position" marker for the CR's actual state, and a faint outline for what's
 * still ahead — with the connecting line itself filling in step by step. The
 * row stretches to the card's full width (each segment is an equal flex
 * share) rather than clumping to one side.
 *
 * `rollback` and `canceled` are deliberately never plotted on this line: both
 * are destructive off-ramps reachable from several different points in the
 * forward path (see `DESTRUCTIVE_TRANSITIONS`), not a 10th/11th sequential
 * step, so forcing either into the line would misrepresent it as "the step
 * after review". When the CR is currently in one of them, the forward line
 * renders dimmed with nothing marked current or complete — the record
 * carries no history of which forward state it was in before the off-ramp —
 * and a distinct tag below names the actual state.
 */
export default function ChangeRequestLifecycleStepper({
  state,
}: {
  state?: string | null;
}): JSX.Element {
  const offRamp = isChangeRequestOffRampState(state);
  const currentIndex = offRamp
    ? -1
    : CHANGE_REQUEST_FORWARD_STATES.indexOf(state as (typeof CHANGE_REQUEST_FORWARD_STATES)[number]);
  const lastIndex = CHANGE_REQUEST_FORWARD_STATES.length - 1;
  // A state the backend could start sending that isn't yet one of the 9
  // forward states or a recognized off-ramp — distinct from "no state at
  // all" (`!state`), which is just a CR still being created and gets no
  // note. Without this, `currentIndex` is -1 the same as an off-ramp but
  // `offRamp` is false, so every marker would render pending with no
  // indication anywhere of what the CR's real state actually is.
  const unrecognizedState = !offRamp && currentIndex < 0 && !!state;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box
        role="list"
        aria-label="Change request lifecycle"
        sx={{ display: "flex", alignItems: "flex-start", opacity: offRamp || unrecognizedState ? 0.45 : 1 }}
      >
        {CHANGE_REQUEST_FORWARD_STATES.map((s, index) => {
          const isCurrent = index === currentIndex;
          const isDone = currentIndex > index;
          // The connector to the LEFT of this node is "filled" once this node
          // itself has been reached (done or current) — so the line completes
          // in step with the marker it leads into, not the one it leads out of.
          const incomingFilled = isDone || isCurrent;
          return (
            <Box
              key={s}
              role="listitem"
              aria-current={isCurrent ? "step" : undefined}
              sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                flex: index === 0 || index === lastIndex ? "0 0 auto" : 1,
                minWidth: 0,
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", width: "100%" }}>
                <Box
                  aria-hidden
                  sx={{
                    flex: 1,
                    height: 2,
                    visibility: index === 0 ? "hidden" : "visible",
                    bgcolor: incomingFilled ? "success.main" : "divider",
                  }}
                />
                <StepNode done={isDone} current={isCurrent} />
                <Box
                  aria-hidden
                  sx={{
                    flex: 1,
                    height: 2,
                    visibility: index === lastIndex ? "hidden" : "visible",
                    bgcolor: isDone ? "success.main" : "divider",
                  }}
                />
              </Box>
              <Typography
                variant="caption"
                align="center"
                sx={{
                  mt: 0.75,
                  maxWidth: 88,
                  lineHeight: 1.25,
                  fontWeight: isCurrent ? 700 : 400,
                  color: isCurrent ? "info.main" : isDone ? "text.primary" : "text.secondary",
                }}
              >
                {changeRequestStateLabel(s)}
              </Typography>
            </Box>
          );
        })}
      </Box>
      {offRamp && (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="caption" color="text.secondary">
            Diverted from the standard path:
          </Typography>
          <Chip size="small" color="error" label={changeRequestStateLabel(state)} />
        </Box>
      )}
      {unrecognizedState && (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="caption" color="text.secondary">
            Current state:
          </Typography>
          <Chip size="small" color="default" label={changeRequestStateLabel(state)} />
        </Box>
      )}
    </Box>
  );
}
