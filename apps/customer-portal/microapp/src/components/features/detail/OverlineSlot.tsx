import { motion, type Variants } from "framer-motion";
import { Skeleton, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import type { ItemCardProps } from "../support";

import { TYPE_CONFIG } from "../support/config";

export function OverlineSlot({
  variant = "normal",
  type,
  id,
  title,
}: {
  variant?: "normal" | "shrunk";
  type: ItemCardProps["type"];
  id?: string;
  title?: string;
}) {
  const { icon: Icon, color } = TYPE_CONFIG[type];

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
        <Typography variant="body1" fontWeight="medium" noWrap>
          {id ?? <Skeleton variant="text" width={120} height={30} />}
        </Typography>
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
