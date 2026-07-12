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

import { Box, pxToRem, styled } from "@wso2/oxygen-ui";
import { useEffect, useRef } from "react";
import { openUrl } from "../microapp-bridge";
import { useQueryClient } from "@tanstack/react-query";
import { cases } from "@root/src/services/cases";

export const RichTextBase = styled(Box)(({ theme }) => ({
  fontSize: pxToRem(13),
  lineHeight: 1.5,

  "& h1": { fontSize: pxToRem(28), margin: 0 },
  "& h2": { fontSize: pxToRem(24), margin: 0 },
  "& h3": { fontSize: pxToRem(20), margin: 0 },
  "& h4": { fontSize: pxToRem(18), margin: 0 },
  "& h5": { fontSize: pxToRem(16), margin: 0 },
  "& h6": { fontSize: pxToRem(14), margin: 0 },

  "& p": { fontSize: pxToRem(13), margin: 0 },

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

  ...theme.applyStyles("dark", {
    color: "#000",
    filter: "invert(1) hue-rotate(180deg) brightness(0.88)",
    "& *": { backgroundColor: "transparent !important" },
    "& pre": { backgroundColor: "#ececec !important", color: "#000 !important" },
    "& img": { filter: "invert(1) hue-rotate(180deg) brightness(1.136)" },
  }),

  "& img": { width: "100%" },
}));

export const RichText = (props: React.ComponentProps<typeof RichTextBase>) => {
  const ref = useRef<HTMLDivElement>(null);
  const queryClient = useQueryClient();

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const handler = (e: MouseEvent) => {
      const anchor = (e.target as HTMLElement).closest("a");
      if (!anchor) return;

      e.preventDefault();
      openUrl({
        url: anchor.href,
        presentationStyle: "FormSheet",
        dismissButtonStyle: "close",
      });
    };

    el.addEventListener("click", handler);
    return () => el.removeEventListener("click", handler);
  }, []);

  useEffect(() => {
    if (!ref.current) return;

    const images = Array.from(ref.current.querySelectorAll<HTMLImageElement>("img[src]"));

    Promise.all(
      images.map(async (image) => {
        image.style.visibility = "hidden";

        const originalSrc = image.getAttribute("src");
        if (!originalSrc) return;
        if (originalSrc.startsWith("blob:") || originalSrc.startsWith("data:")) return;

        const id = originalSrc.split("/").pop()?.split(".")[0];
        if (!id) return;

        try {
          const { content: data } = await queryClient.fetchQuery(cases.attachment(id));
          if (!data) return;
          image.src = data;
        } finally {
          image.style.visibility = "visible";
        }
      }),
    );
  }, [props.dangerouslySetInnerHTML]);

  return <RichTextBase ref={ref} {...props} />;
};
