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
import { Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { motion, type Transition, type Variants } from "framer-motion";

import { CASE_TYPE_CONFIGS } from "@shared/constants";
import type { CaseType } from "@shared/types";

type SlotTitleProps = {
  variant?: "default" | "shrunk";
  type: CaseType;
  id?: string;
  title?: string;
};

export function SlotTitle({ variant = "default", type, id, title }: SlotTitleProps) {
  const { icon: Icon, color } = CASE_TYPE_CONFIGS[type];

  return (
    <Stack height={40}>
      <MetaRow icon={<Icon color={color} size={20} />} id={id} variant={variant} />
      {variant === "shrunk" && <TitleRow title={title} />}
    </Stack>
  );
}

function MetaRow({ icon, id, variant }: { icon: React.ReactNode; id?: string; variant: SlotTitleProps["variant"] }) {
  return (
    <Stack
      component={motion.div}
      variants={metaRowVariants}
      animate={variant}
      sx={{
        flexDirection: "row",
        flexGrow: 1,
        gap: 1,
        alignItems: "center",
        transformOrigin: "top left",
        minWidth: 0,
      }}
    >
      {icon}
      <Typography variant="body1" fontWeight="medium" noWrap>
        {id ?? <Skeleton variant="text" width={120} height={30} />}
      </Typography>
    </Stack>
  );
}

function TitleRow({ title }: { title?: string }) {
  return (
    <motion.div variants={titleRowVariants} initial="initial" animate="enter" exit="exit">
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
  );
}

const springTransition: Transition = { type: "spring", stiffness: 300, damping: 50 };

const metaRowVariants: Variants = {
  default: { opacity: 1, scale: 1, y: 0, transition: springTransition },
  shrunk: { opacity: 0.8, scale: 0.8, y: -2, transition: springTransition },
};

const titleRowVariants: Variants = {
  initial: { opacity: 0 },
  enter: { opacity: 1, transition: { duration: 0.2, ease: "easeOut" } },
  exit: { opacity: 0, transition: { duration: 0.2, ease: "easeIn" } },
};
