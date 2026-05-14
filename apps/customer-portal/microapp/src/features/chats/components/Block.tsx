import type { MessageBlock } from "@features/chats/types";
import { TypewriterText } from "@root/src/shared/components/common";
import { Divider, Stack, Typography } from "@wso2/oxygen-ui";
import { ChecklistItem } from "./ChecklistItem";
import { KBCard } from "./KBCard";

export interface BlockProps {
    block: MessageBlock;
    animated: boolean;
    thinking: string | false;
    onAnimationComplete?: () => void;
  }
  
export function Block({ block, animated, thinking, onAnimationComplete }: BlockProps) {
    switch (block.type) {
      case "text":
        return (
          <Typography variant="body2" component="span" sx={{ "& > *": { margin: 0, lineHeight: 1.7 } }}>
            <TypewriterText
              tokens={block.value.split("")}
              animated={animated}
              pending={!!thinking}
              onAnimationComplete={onAnimationComplete}
            />
          </Typography>
        );
  
      case "checklist":
        return (
          <Stack gap={1}>
            {block.items.map((item, i) => (
              <ChecklistItem key={i}>{item}</ChecklistItem>
            ))}
          </Stack>
        );
  
      case "kb":
        return (
          <Stack gap={1.5}>
            <Divider sx={{ my: 1 }} />
            {block.items.map((item, i) => (
              <KBCard key={i} id={item.id} title={item.title} />
            ))}
          </Stack>
        );
  
      default:
        return null;
    }
  }
  