import { Box, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import type {
  HealthState,
  LifecycleCatalogue,
  LifecycleStage,
} from "@features/plg/api/types";

/**
 * The lifecycle, drawn from the catalogue the backend serves.
 *
 * THE LAYOUT IS COMPUTED, NOT AUTHORED. The lifecycle is a line: five stages in
 * order, with ABANDONED hanging off all of them. A line can be laid out from the
 * stage order alone, so adding or renaming a stage needs no change here.
 *
 * Every stage is drawn identically. Colour is spent on ONE thing: the node the
 * pairing is standing on, painted green when the account is healthy and red
 * when it is at risk. Reading a stage as good or bad is the engineer's job, not
 * the palette's — COMMERCIAL is not success and REGISTRATION is not failure, so
 * neither gets a colour of its own.
 *
 * That the position and the health share a single mark is the point. They are
 * the two things the diagram is asked at a glance, and one red box answers both
 * without the reader pairing up a node with a swatch somewhere else.
 *
 * IT DRAWS ONE PAIRING AND NOTHING ELSE. Each node once carried a caption
 * saying how many pairings sat at that stage, taken from the catalogue. That
 * was portfolio data on a page about a single customer: the diagram exists to
 * answer "where does THIS pairing stand", and a node reading "2 pairings"
 * invites the reader to attach that number to the account in front of them. The
 * count is gone from the API as well — see plg_reference_repo.go — so there is
 * nothing here to render even by accident.
 *
 * Stage-by-stage totals are a real question; they belong to the dashboard,
 * which is scoped and labelled as a portfolio view.
 */

const NODE_WIDTH = 168;
const NODE_HEIGHT = 54;
const RADIUS = 12;
const GAP = 40;
const ROW_Y = 62;
const TERMINAL_Y = 196;
const MARGIN = 24;

/** Where each stage sits, derived from its place in the progression. */
function layout(stages: LifecycleStage[]) {
  const pos = new Map<LifecycleStage, { x: number; y: number }>();
  const progression = stages.filter((s) => s !== "ABANDONED");

  progression.forEach((stage, i) => {
    pos.set(stage, {
      x: MARGIN + NODE_WIDTH / 2 + i * (NODE_WIDTH + GAP),
      y: ROW_Y,
    });
  });

  // ABANDONED sits below the middle of the run, not after COMMERCIAL. It is not
  // the step that follows the last stage — it is reachable from every one of
  // them, and drawing it in line would say the opposite.
  const midIndex = (progression.length - 1) / 2;
  pos.set("ABANDONED", {
    x: MARGIN + NODE_WIDTH / 2 + midIndex * (NODE_WIDTH + GAP),
    y: TERMINAL_Y,
  });

  const width = MARGIN * 2 + progression.length * NODE_WIDTH + (progression.length - 1) * GAP;
  return { pos, progression, width };
}

export function StageFlow({
  catalogue,
  currentStage,
  healthState,
  height = 260,
}: {
  catalogue: LifecycleCatalogue;
  currentStage: LifecycleStage;
  /** Colours the current node: green healthy, red at risk. Required, because
   *  this diagram only ever draws one pairing — there is no abstract mode. */
  healthState: HealthState;
  height?: number;
}) {
  const theme = useTheme();

  const line = theme.palette.divider;
  const lineStrong = theme.palette.text.disabled;
  const healthy = theme.palette.success.main;
  const atRiskColour = theme.palette.error.main;

  const ordered = [...catalogue.stages].sort((a, b) => a.displayOrder - b.displayOrder);
  const { pos, progression, width } = layout(ordered.map((s) => s.stage));
  const viewHeight = TERMINAL_Y + NODE_HEIGHT / 2 + MARGIN;

  const atRisk = healthState === "AT_RISK";
  const currentColour = atRisk ? atRiskColour : healthy;

  return (
    <Box>
      <Box sx={{ width: "100%", overflowX: "auto" }}>
        <svg
          viewBox={`0 0 ${width} ${viewHeight}`}
          width="100%"
          height={height}
          role="img"
          aria-label="PLG lifecycle stages, in the order a pairing moves through them"
        >
          <defs>
            <marker
              id="stage-arrow"
              viewBox="0 0 10 10"
              refX="9"
              refY="5"
              markerWidth="5"
              markerHeight="5"
              orient="auto-start-reverse"
            >
              <path d="M 0 0 L 10 5 L 0 10 z" fill={lineStrong} />
            </marker>
          </defs>

          {/* The progression: one arrow per step, between adjacent boxes. */}
          {progression.slice(0, -1).map((stage, i) => {
            const from = pos.get(stage)!;
            const to = pos.get(progression[i + 1])!;
            return (
              <line
                key={`${stage}-arrow`}
                x1={from.x + NODE_WIDTH / 2 + 6}
                y1={from.y}
                x2={to.x - NODE_WIDTH / 2 - 10}
                y2={to.y}
                stroke={lineStrong}
                strokeWidth={1.5}
                markerEnd="url(#stage-arrow)"
              />
            );
          })}

          {/* One dashed line down to ABANDONED, labelled rather than repeated.
              Drawing an arrow from every stage would be accurate and unreadable;
              the words carry it instead. */}
          <line
            x1={pos.get("ABANDONED")!.x}
            y1={ROW_Y + NODE_HEIGHT / 2 + 6}
            x2={pos.get("ABANDONED")!.x}
            y2={TERMINAL_Y - NODE_HEIGHT / 2 - 10}
            stroke={line}
            strokeWidth={1.5}
            strokeDasharray="4 4"
            markerEnd="url(#stage-arrow)"
          />
          <text
            x={pos.get("ABANDONED")!.x + 12}
            y={(ROW_Y + TERMINAL_Y) / 2}
            fontSize={11}
            fill={theme.palette.text.secondary}
            dominantBaseline="middle"
          >
            from any stage
          </text>

          {ordered.map((info) => {
            const p = pos.get(info.stage)!;
            const isCurrent = info.stage === currentStage;
            return (
              <g key={info.stage}>
                <rect
                  x={p.x - NODE_WIDTH / 2}
                  y={p.y - NODE_HEIGHT / 2}
                  width={NODE_WIDTH}
                  height={NODE_HEIGHT}
                  rx={RADIUS}
                  fill={isCurrent ? currentColour : theme.palette.background.paper}
                  stroke={isCurrent ? currentColour : line}
                  strokeWidth={isCurrent ? 2 : 1}
                />
                <text
                  x={p.x}
                  y={p.y}
                  textAnchor="middle"
                  dominantBaseline="middle"
                  fontSize={13}
                  fontWeight={isCurrent ? 600 : 500}
                  fill={
                    isCurrent
                      ? theme.palette.getContrastText(currentColour)
                      : theme.palette.text.primary
                  }
                >
                  {info.name}
                </text>
              </g>
            );
          })}
        </svg>
      </Box>

      <Stack direction="row" spacing={2} sx={{ mt: 1, flexWrap: "wrap" }} useFlexGap>
        <Legend
          colour={currentColour}
          label={atRisk ? "Here, and at risk" : "Here, and healthy"}
        />
        <Typography variant="caption" color="text.secondary">
          A pairing moves forward only. Abandoned is reachable from anywhere and
          nothing leaves it.
        </Typography>
      </Stack>
    </Box>
  );
}

function Legend({ colour, label }: { colour: string; label: string }) {
  return (
    <Stack direction="row" spacing={0.75} alignItems="center">
      <Box sx={{ width: 12, height: 12, borderRadius: "3px", bgcolor: colour }} />
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
    </Stack>
  );
}
