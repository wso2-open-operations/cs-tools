// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Button, TextField, Typography } from "@wso2/oxygen-ui";
import { useRef, useEffect, useState, type JSX } from "react";
import { htmlToMarkdown, markdownToHtml } from "@utils/richTextEditor";

export interface MarkdownSectionProps {
  value: string;
  onChange: (html: string) => void;
  disabled?: boolean;
}

/**
 * Markdown editor section for inline markdown editing.
 *
 * @param {MarkdownSectionProps} props - Component props.
 * @returns {JSX.Element} The rendered markdown section.
 */
export function MarkdownEditorSection({
  value,
  onChange,
  disabled = false,
}: MarkdownSectionProps): JSX.Element {
  const [markdown, setMarkdown] = useState(() => htmlToMarkdown(value ?? ""));
  const isEditingRef = useRef(false);

  useEffect(() => {
    if (!isEditingRef.current) {
      setMarkdown(htmlToMarkdown(value ?? ""));
    }
  }, [value]);

  const handleFocus = () => {
    isEditingRef.current = true;
  };

  const handleBlur = () => {
    isEditingRef.current = false;
  };

  const handleApply = () => {
    const html = markdownToHtml(markdown) || "";
    onChange(html);
    isEditingRef.current = false;
    setMarkdown(htmlToMarkdown(html));
  };

  return (
    <Box
      sx={{
        borderTop: "1px solid",
        borderColor: "divider",
        bgcolor: "background.default",
        p: 2,
      }}
    >
      <Typography variant="subtitle2" sx={{ mb: 1.5 }}>
        Edit as Markdown
      </Typography>
      <TextField
        multiline
        fullWidth
        minRows={8}
        maxRows={16}
        value={markdown}
        onChange={(e) => setMarkdown(e.target.value)}
        onFocus={handleFocus}
        onBlur={handleBlur}
        disabled={disabled}
        placeholder="Write your content in Markdown..."
        sx={{
          "& textarea": {
            fontFamily: "monospace",
            fontSize: "0.875rem",
          },
        }}
      />
      <Button
        size="small"
        variant="contained"
        onClick={handleApply}
        disabled={disabled}
        sx={{ mt: 1.5 }}
      >
        Apply to editor
      </Button>
    </Box>
  );
}
