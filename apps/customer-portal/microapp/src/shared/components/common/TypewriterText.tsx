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
import { useEffect, useRef, useState } from "react";

import { Box, colors } from "@wso2/oxygen-ui";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface TypewriterProps {
  tokens: string[];
  pending?: boolean;
  animated?: boolean;
  onAnimationComplete?: () => void;
}

export function TypewriterText({ tokens, pending = false, animated = true, onAnimationComplete }: TypewriterProps) {
  const fullText = tokens.join("");
  const [cursor, setCursor] = useState(0);
  const hasCompletedRef = useRef(false);

  useEffect(() => {
    if (!animated) return;

    if (cursor >= fullText.length) {
      if (!hasCompletedRef.current) {
        hasCompletedRef.current = true;
        onAnimationComplete?.();
      }
      return;
    }

    const t = setTimeout(() => setCursor((c) => c + 1), 30);

    return () => clearTimeout(t);
  }, [cursor, fullText, animated, onAnimationComplete]);

  useEffect(() => {
    if (animated) {
      hasCompletedRef.current = false;
      setCursor(0);
    }
  }, [fullText, animated]);

  return (
    <Box sx={{ "& p": { display: "inline", m: 0 } }}>
      <Markdown
        remarkPlugins={[remarkGfm]}
        components={{
          table: ({ node, ...props }) => (
            <div style={{ overflowX: "auto", marginTop: 10 }}>
              <table
                style={{
                  width: "max-content",
                  border: `1px solid ${colors.grey[500]}`,
                  borderCollapse: "collapse",
                }}
                {...props}
              />
            </div>
          ),
          th: ({ node, ...props }) => <th style={{ border: `1px solid ${colors.grey[500]}` }} {...props} />,
          td: ({ node, ...props }) => (
            <td style={{ padding: 5, border: `1px solid ${colors.grey[500]}`, maxWidth: 500 }} {...props} />
          ),
        }}
      >
        {animated ? fullText.slice(0, cursor) : fullText}
      </Markdown>
      <Box
        sx={{
          display: pending || (animated && cursor < fullText.length) ? "inline-block" : "none",
          width: 15,
          height: 15,
          borderRadius: "100%",
          bgcolor: "primary.main",
          ml: 0.5,
          verticalAlign: "middle",
          animation: "blink 1s infinite",
        }}
      />
    </Box>
  );
}
