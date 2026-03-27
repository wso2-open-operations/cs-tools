import { Box, pxToRem, styled } from "@wso2/oxygen-ui";

export const RichText = styled(Box)(({ theme }) => ({
  fontSize: pxToRem(13),
  lineHeight: 1.5,

  "& h1": { fontSize: pxToRem(28) },
  "& h2": { fontSize: pxToRem(24) },
  "& h3": { fontSize: pxToRem(20) },
  "& h4": { fontSize: pxToRem(18) },
  "& h5": { fontSize: pxToRem(16) },
  "& h6": { fontSize: pxToRem(14) },

  "& p": { fontSize: pxToRem(13) },

  "& pre": {
    padding: 12,
    fontSize: pxToRem(12),
    fontFamily: "monospace",
    overflowX: "auto",
    whiteSpace: "pre-wrap",
    backgroundColor: theme.palette.background.default,
    color: theme.palette.text.primary,
    borderRadius: 4,
  },

  "& p:has(br:only-child)": { display: "none" },

  // force all inline spans to inherit
  "& span": { fontSize: "inherit !important", lineHeight: "inherit !important" },
  "& strong, & em": { fontSize: "inherit !important", lineHeight: "inherit !important" },
  "& pre span": {
    fontSize: "inherit !important",
    fontFamily: "inherit !important",
    lineHeight: "inherit !important",
  },
}));
