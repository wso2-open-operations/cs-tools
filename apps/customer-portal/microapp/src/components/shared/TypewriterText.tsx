import { Box } from "@wso2/oxygen-ui";
import { useEffect, useState } from "react";
import Markdown from "react-markdown";

interface TypewriterProps {
  tokens: string[];
  pending?: boolean;
  animated?: boolean;
}

export function TypewriterText({ tokens, pending = false, animated = true }: TypewriterProps) {
  const fullText = tokens.join("");
  const [cursor, setCursor] = useState(0);

  useEffect(() => {
    if (cursor >= fullText.length) return;
    const t = setTimeout(() => setCursor((c) => c + 1), 50);
    return () => clearTimeout(t);
  }, [cursor, fullText, animated]);

  return (
    <>
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

      <style>{`
        @keyframes blink { 50% { opacity: 0; } }
        p { display: inline; margin: 0; }
      `}</style>
    </>
  );
}
