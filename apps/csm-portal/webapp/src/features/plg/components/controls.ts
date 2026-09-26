/**
 * Sizes shared by the editors that stage a change behind a Save button.
 *
 * A plain module rather than part of `common.tsx` because a `.tsx` file that
 * exports both components and values breaks Vite's fast refresh.
 */

/** 40px is the height of a size="small" TextField, so buttons line up with the
 *  inputs rather than standing a few pixels taller than them. */
export const AXIS_BUTTON_SX = { height: 40, whiteSpace: "nowrap" } as const;

/** Every picker that sits above a reason box is the same width, so the boxes
 *  start on the same vertical line down a card. */
export const CONTROL_WIDTH = 260;
