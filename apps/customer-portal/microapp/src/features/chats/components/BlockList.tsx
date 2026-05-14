import { Block, type BlockProps } from "@features/chats/components";
import type { MessageBlock } from "@features/chats/types";
import { Stack } from "@wso2/oxygen-ui";

type BlockListProps = Omit<BlockProps, "block"> & { blocks: MessageBlock[] };

/**
 * NOTE: Currently, messages only ever contain a single text block, so multi-block
 * rendering is effectively untested. If support for multiple blocks per message is
 * added in the future, this component (and the `Block` renderer) will need to be
 * revisited — particularly around animation sequencing and key stability.
 */
export function BlockList({ blocks, ...blockProps }: BlockListProps) {
  return (
    <Stack gap={2}>
      {blocks.map((block, index) => (
        <Block key={index} block={block} {...blockProps} />
      ))}
    </Stack>
  );
}