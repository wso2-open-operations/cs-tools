import { Box } from "@wso2/oxygen-ui";
import { useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";

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

    const t = setTimeout(() => setCursor((c) => c + 1), 50);

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
      <Markdown>{animated ? fullText.slice(0, cursor) : fullText}</Markdown>
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
