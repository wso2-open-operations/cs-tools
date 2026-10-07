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

import { Box, Chip, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Ban, Check, Undo2, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useEffect, useRef } from "react";
import type { BeChangeRequestApproval } from "@api/backend/types";
import { changeRequestStateLabel, isChangeRequestOffRampState } from "@features/csm-operations/utils/changeRequests";
import {
  buildChangeRequestLifecycle,
  changeRequestLifecycleStatusText,
  isChangeRequestLifecycleState,
  type ChangeRequestLifecycleNode,
  type ChangeRequestLifecycleStatus,
} from "@features/csm-operations/utils/changeRequestStages";

const NODE_SIZE = 22;
/** How faint a stage's MARKER reads when it is not part of this change's path (the customer portal's disabled stage is 0.5). */
const MUTED_OPACITY = 0.45;
/**
 * How strong an upcoming stage's ring reads: less than a done or current marker
 * (they are filled) and clearly more than a not-taken one. At full strength the
 * secondary text colour is 9:1 against the page, heavier than a to-do ring needs;
 * at this it stays above the 3:1 a state-conveying graphic needs on both themes.
 */
const PENDING_OPACITY = 0.7;
/**
 * How faint such a stage's LABEL reads. Much less than its marker: the label is
 * text, and the secondary text colour dimmed to the marker's level (or 0.6, as
 * it was) falls below the 4.5:1 text contrast on the light theme's page
 * backdrop. The dashed ring and the icon carry the "not part of this path" cue.
 */
const MUTED_LABEL_OPACITY = 0.85;
/**
 * Narrowest a stage may get. The row is `stages x` this wide at least: a
 * container narrower than that scrolls the row inside itself instead of
 * squashing the labels (the longest single word, "Authorize" / "Implement",
 * is about 58px at the caption size).
 */
const MIN_NODE_WIDTH = 68;

/** Read by assistive tech, invisible to everyone else. Pixel strings on purpose: in `sx` a bare `1` means 100%. */
const visuallyHidden = {
  position: "absolute",
  width: "1px",
  height: "1px",
  p: 0,
  m: "-1px",
  overflow: "hidden",
  clip: "rect(0 0 0 0)",
  whiteSpace: "nowrap",
  border: 0,
} as const;

/** Rollback and Canceled are the two stages a change reaches only by leaving the path. */
function ExceptionIcon({ stage, size }: { stage: string; size: number }): JSX.Element | null {
  if (stage === "rollback") return <Undo2 size={size} strokeWidth={2.5} />;
  if (stage === "canceled") return <Ban size={size} strokeWidth={2.5} />;
  return null;
}

/**
 * One step marker, by what the stage means for this change:
 *  - done: a filled success circle with a check;
 *  - current: a filled circle plus a soft halo (a larger, low-opacity ring
 *    behind it) so it reads as "you are here" at a glance. Info for a stage
 *    on the path, error for Rollback / Canceled (with their icon);
 *  - pending: an outlined circle;
 *  - unrecorded: the same outline, faint (the record cannot say);
 *  - not-taken: a faint DASHED outline, carrying the stage's icon when it is
 *    Rollback / Canceled so it is still recognisable;
 *  - rejected: an error-coloured outline with a cross (the customer said no
 *    here; the change ended in the Rollback / Canceled stage after it).
 *
 * Current uses `info` rather than `primary`: `primary` is this app's brand
 * accent, already used everywhere (buttons, the active tab underline, links)
 * — reusing it here would blend the stepper into that noise instead of
 * standing out as its own "you are here" signal. `info` reads as
 * in-progress/informational and doesn't compete with any nearby primary CTA.
 */
function StepNode({ stage, status }: { stage: string; status: ChangeRequestLifecycleStatus }): JSX.Element {
  const exception = isChangeRequestOffRampState(stage);
  const tone = exception ? "error" : "info";
  const faint = status === "not-taken" || status === "unrecorded";
  return (
    <Box sx={{ position: "relative", display: "flex", alignItems: "center", justifyContent: "center" }}>
      {status === "current" && (
        <Box
          aria-hidden
          sx={{
            position: "absolute",
            width: NODE_SIZE + 12,
            height: NODE_SIZE + 12,
            borderRadius: "50%",
            bgcolor: `${tone}.main`,
            opacity: 0.15,
          }}
        />
      )}
      <Box
        aria-hidden
        sx={{
          position: "relative",
          width: NODE_SIZE,
          height: NODE_SIZE,
          borderRadius: "50%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          flexShrink: 0,
          boxSizing: "border-box",
          transition: "background-color 0.15s, border-color 0.15s",
          opacity: faint ? MUTED_OPACITY : status === "pending" ? PENDING_OPACITY : 1,
          ...(status === "done"
            ? { bgcolor: "success.main", color: "success.contrastText" }
            : status === "current"
              ? { bgcolor: `${tone}.main`, color: `${tone}.contrastText` }
              : status === "rejected"
                ? { bgcolor: "transparent", border: "2px solid", borderColor: "error.main", color: "error.main" }
                : {
                    bgcolor: "transparent",
                    border: "2px",
                    borderStyle: status === "not-taken" ? "dashed" : "solid",
                    // Text-secondary, not the divider colour (1.2:1): an upcoming stage's
                    // ring is a state-conveying graphic and needs 3:1 against the page,
                    // and it must stay clearer than a not-taken one (dimmed above).
                    borderColor: "text.secondary",
                    color: "text.secondary",
                  }),
        }}
      >
        {status === "done" ? (
          <Check size={13} strokeWidth={3} />
        ) : status === "current" ? (
          exception ? (
            <ExceptionIcon stage={stage} size={13} />
          ) : (
            <Box sx={{ width: 7, height: 7, borderRadius: "50%", bgcolor: "info.contrastText" }} />
          )
        ) : status === "rejected" ? (
          <X size={12} strokeWidth={3} />
        ) : status === "not-taken" && exception ? (
          <ExceptionIcon stage={stage} size={12} />
        ) : null}
      </Box>
    </Box>
  );
}

type Segment = "filled" | "error" | "dashed" | "plain";

/**
 * How the connector LEADING INTO stage `index` is drawn (both halves of it,
 * the one ending at the previous node and the one starting at this one, use
 * the same answer so a segment never changes colour mid-way):
 *  - filled: the line is completed up to here (done / current / rejected stage);
 *  - error: into the current Rollback / Canceled;
 *  - dashed: into a stage that is not part of this change's path;
 *  - plain: the line still to be filled.
 * Rollback sits ON the line between Customer Review and Closed although a
 * change that goes to plan never visits it: when the change carried on past
 * it, the line is drawn filled right through the faint Rollback node rather
 * than looking broken there.
 */
function segmentInto(nodes: readonly ChangeRequestLifecycleNode[], index: number): Segment {
  const node = nodes[index]!;
  const exception = isChangeRequestOffRampState(node.key);
  switch (node.status) {
    case "current":
      return exception ? "error" : "filled";
    case "done":
    case "rejected":
      return "filled";
    case "not-taken": {
      if (!exception) return "dashed";
      const next = nodes.slice(index + 1).find((n) => !isChangeRequestOffRampState(n.key));
      return next && (next.status === "done" || next.status === "current") ? "filled" : "dashed";
    }
    default:
      return "plain";
  }
}

function Connector({ segment, hidden }: { segment: Segment; hidden: boolean }): JSX.Element {
  return (
    <Box
      aria-hidden
      data-segment={segment}
      sx={{
        flex: 1,
        height: 0,
        visibility: hidden ? "hidden" : "visible",
        borderTop: "2px",
        borderTopStyle: segment === "dashed" ? "dashed" : "solid",
        borderTopColor: segment === "filled" ? "success.main" : segment === "error" ? "error.main" : "divider",
      }}
    />
  );
}

type Tone = "info" | "error";

/**
 * Typed structurally, as `CaseTabStrip` does: these members are all this needs
 * and MUI hands the real theme over at call time.
 */
type ColorSchemeAwareTheme = {
  palette: Record<Tone, { main: string; dark: string; light: string }>;
  applyStyles: (scheme: "dark" | "light", styles: Record<string, unknown>) => Record<string, unknown>;
};

/**
 * A tone's text colour: the dark shade on the light theme and the light shade
 * on the dark one. The main shade is below 4.5:1 at caption size on both (about
 * 3.3:1 on the light page backdrop, 3.8 to 4.1:1 for the error red on the dark
 * one).
 */
function toneText(tone: Tone): (theme: ColorSchemeAwareTheme) => Record<string, unknown> {
  return (theme) => ({
    color: theme.palette[tone].dark,
    ...theme.applyStyles("dark", { color: theme.palette[tone].light }),
  });
}

function labelColorSx(
  status: ChangeRequestLifecycleStatus,
  exception: boolean,
): { color: string } | ((theme: ColorSchemeAwareTheme) => Record<string, unknown>) {
  if (status === "current") return toneText(exception ? "error" : "info");
  if (status === "rejected") return toneText("error");
  return { color: status === "done" ? "text.primary" : "text.secondary" };
}

/**
 * Horizontal lifecycle indicator for a change request: the customer portal's
 * eleven-stage workflow (New, Assess, Authorize, Customer Approval, Scheduled,
 * Implement, Review, Customer Review, Rollback, Closed, Canceled) laid out as a
 * connected line of step markers, with the connecting line filling in step by
 * step. What each stage says about THIS change comes from
 * {@link buildChangeRequestLifecycle} (see it for the rules): stages passed are
 * checked, the CR's state is highlighted, what is still ahead is outlined, and
 * Rollback / Canceled — the two exits off the path, which most changes never
 * take — are plotted in the customer portal's place for them but read as "not
 * taken" (faint, dashed) until the change actually ends in one, when that stage
 * turns error-coloured with its icon. When the customer's rejection is what
 * ended the change (Customer Approval -> Canceled, Customer Review -> Rollback)
 * the rejected stage shows a cross in the error colour, and the stages after it
 * on a canceled change read "not taken": the record proves they were never reached.
 *
 * Each stage names its state in words as well as colour (a visually-hidden
 * "done" / "current" / "not taken" ...), and carries the customer portal's
 * caption for it as a tooltip and an accessible description. The row is as
 * wide as the page allows, each stage an equal share; on a viewport too narrow
 * for eleven readable stages it scrolls inside its own container, centred on
 * the current stage, instead of squashing the labels or scrolling the page.
 *
 * A state that is not one of the eleven (a state the backend could start
 * sending) is not plotted anywhere: the line renders faint with nothing marked
 * and a tag below names the actual state — distinct from "no state at all",
 * which is just a change still being created and gets no note.
 */
export default function ChangeRequestLifecycleStepper({
  state,
  customerApprovalRequired,
  customerReviewRequired,
  approvals,
  customerApproved,
  hasCustomerContacts,
}: {
  state?: string | null;
  customerApprovalRequired?: boolean;
  customerReviewRequired?: boolean;
  /** `GET /change-requests/{id}/approvals`, when loaded: evidence for a rolled-back or canceled change. */
  approvals?: readonly Pick<BeChangeRequestApproval, "stage" | "status">[];
  /** The change's `hasCustomerApproved`: more evidence for a canceled change. */
  customerApproved?: boolean;
  /** Whether the change's project has registered customer contacts (`undefined` = unknown): more evidence for a rolled-back change. */
  hasCustomerContacts?: boolean;
}): JSX.Element {
  const nodes = buildChangeRequestLifecycle({
    state,
    customerApprovalRequired,
    customerReviewRequired,
    approvals,
    customerApproved,
    hasCustomerContacts,
  });
  const unrecognizedState = !!state && !isChangeRequestLifecycleState(state);
  const lastIndex = nodes.length - 1;

  // Keep the current stage in view when the row has to scroll (a narrow viewport).
  const scrollerRef = useRef<HTMLDivElement>(null);
  const currentKey = nodes.find((n) => n.status === "current")?.key;
  useEffect(() => {
    const scroller = scrollerRef.current;
    const current = scroller?.querySelector<HTMLElement>('[aria-current="step"]');
    if (!scroller || !current || scroller.scrollWidth <= scroller.clientWidth) return;
    scroller.scrollLeft = current.offsetLeft - (scroller.clientWidth - current.offsetWidth) / 2;
  }, [currentKey]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
      {/* A printed page cannot scroll: there the row gives up its minimum width and fits the page. */}
      <Box
        ref={scrollerRef}
        sx={{ position: "relative", overflowX: "auto", pb: 0.5, "@media print": { overflow: "visible" } }}
      >
        <Box
          role="list"
          aria-label="Change request lifecycle"
          sx={{
            display: "flex",
            alignItems: "flex-start",
            minWidth: nodes.length * MIN_NODE_WIDTH,
            opacity: unrecognizedState ? 0.45 : 1,
            "@media print": { minWidth: 0 },
          }}
        >
          {nodes.map((node, index) => {
            const exception = isChangeRequestOffRampState(node.key);
            const isCurrent = node.status === "current";
            const into = segmentInto(nodes, index);
            const after = index < lastIndex ? segmentInto(nodes, index + 1) : "plain";
            const faint = node.status === "not-taken" || node.status === "unrecorded";
            // The caption, and for the three statuses a glance at the line does not
            // explain (a dashed or faint marker, a cross) what they mean.
            const hint =
              faint || node.status === "rejected"
                ? `${node.caption} (${changeRequestLifecycleStatusText(node.status)})`
                : node.caption;
            return (
              <Tooltip key={node.key} title={hint} placement="bottom" arrow describeChild enterDelay={300}>
                <Box
                  role="listitem"
                  aria-current={isCurrent ? "step" : undefined}
                  aria-description={node.caption}
                  sx={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    flex: index === 0 || index === lastIndex ? "0 0 auto" : "1 1 0",
                    minWidth: index === 0 || index === lastIndex ? undefined : MIN_NODE_WIDTH,
                  }}
                >
                  <Box sx={{ display: "flex", alignItems: "center", width: "100%" }}>
                    <Connector segment={into} hidden={index === 0} />
                    <StepNode stage={node.key} status={node.status} />
                    <Connector segment={after} hidden={index === lastIndex} />
                  </Box>
                  <Typography
                    variant="caption"
                    align="center"
                    sx={[
                      {
                        mt: 0.75,
                        maxWidth: 88,
                        lineHeight: 1.25,
                        fontWeight: isCurrent ? 700 : 400,
                        opacity: faint ? MUTED_LABEL_OPACITY : 1,
                      },
                      labelColorSx(node.status, exception),
                    ]}
                  >
                    {node.label}
                    <Box component="span" sx={visuallyHidden}>
                      , {changeRequestLifecycleStatusText(node.status)}
                    </Box>
                  </Typography>
                </Box>
              </Tooltip>
            );
          })}
        </Box>
      </Box>
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
