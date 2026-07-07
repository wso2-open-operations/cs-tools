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
import { motion, type Variants } from "framer-motion";
import { IconButton, Skeleton, Stack, Tooltip, Typography, pxToRem } from "@wso2/oxygen-ui";
import { Check, Copy } from "@wso2/oxygen-ui-icons-react";
import type { ItemCardProps } from "../support";

import { TYPE_CONFIG } from "../support/config";

export function OverlineSlot({
  variant = "normal",
  type,
  id,
  ids,
  title,
}: {
  variant?: "normal" | "shrunk";
  type: ItemCardProps["type"];
  id?: string;
  /** One or more copyable identifiers to render instead of `id` (e.g. internal + external case IDs). */
  ids?: string[];
  title?: string;
}) {
  const { icon: Icon, color } = TYPE_CONFIG[type];
  const [copied, setCopied] = useState(false);
  const [hovered, setHovered] = useState(false);

  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
      copyTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access denied or unavailable — nothing actionable to do here.
    }
  };

  useEffect(() => {
    return () => {
      if (copyTimeoutRef.current) clearTimeout(copyTimeoutRef.current);
    };
  }, []);

  return (
    <Stack height={40}>
      <Stack
        sx={{
          flexDirection: "row",
          flexGrow: 1,
          gap: 1,
          alignItems: "center",
          transformOrigin: "top left",
          minWidth: 0,
        }}
        component={motion.div}
        variants={containerVariants}
        animate={variant}
      >
        <Icon color={color} size={pxToRem(16)} />
        {ids ? (
          <Stack direction="row" alignItems="center" gap={0.5} minWidth={0}>
            <Typography variant="body1" fontWeight="medium" noWrap>
              {ids.join(" | ")}
            </Typography>
            <Tooltip
              title={copied ? "Copied!" : "Copy"}
              open={copied || hovered}
              onOpen={() => setHovered(true)}
              onClose={() => setHovered(false)}
            >
              <IconButton
                size="small"
                color={copied ? "success" : "default"}
                onClick={() => handleCopy(ids.join(" | "))}
                aria-label="Copy IDs"
              >
                {copied ? <Check size={pxToRem(16)} /> : <Copy size={pxToRem(16)} />}
              </IconButton>
            </Tooltip>
          </Stack>
        ) : (
          <Typography variant="body1" fontWeight="medium" noWrap>
            {id ?? <Skeleton variant="text" width={120} height={30} />}
          </Typography>
        )}
      </Stack>

      {variant === "shrunk" && (
        <motion.div variants={titleVariants} initial="initial" animate="enter" exit="exit">
          <Typography
            variant="body1"
            fontWeight="medium"
            mt={-1}
            sx={{
              overflow: "hidden",
              textOverflow: "ellipsis",
              display: "-webkit-box",
              WebkitLineClamp: 1,
              WebkitBoxOrient: "vertical",
              wordBreak: "break-word",
            }}
          >
            {title ?? <Skeleton variant="text" width="100%" height={30} />}
          </Typography>
        </motion.div>
      )}
    </Stack>
  );
}

// Animations
const containerVariants: Variants = {
  normal: {
    opacity: 1,
    scale: 1,
    y: 0,
    transition: { type: "spring", stiffness: 300, damping: 30 },
  },
  shrunk: {
    opacity: 0.8,
    scale: 0.8,
    y: -2,
    transition: { type: "spring", stiffness: 300, damping: 30 },
  },
};

const titleVariants: Variants = {
  initial: { opacity: 0 },
  enter: { opacity: 1, transition: { duration: 0.2, ease: "easeOut" } },
  exit: { opacity: 0, transition: { duration: 0.2, ease: "easeIn" } },
};
