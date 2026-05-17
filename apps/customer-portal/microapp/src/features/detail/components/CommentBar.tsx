import { useEffect, useRef, useState } from "react";

import { Box } from "@wso2/oxygen-ui";

import { useCase, useComments } from "@features/detail/hooks";

import { CommentBar as BaseCommentBar } from "@shared/components/core";

import { scrollTo } from "@shared/utils";

export function CommentBar() {
  const { data } = useCase();
  const [comment, setComment] = useState("");
  const { comments, submit, isPending } = useComments();
  const bottomRef = useRef<HTMLDivElement>(null);

  const handleSend = () => {
    submit(comment);
    setComment("");
  };

  useEffect(() => scrollTo(bottomRef), [comments]);

  return (
    <>
      {/* Marks the end of the comment list; used as a scroll target */}
      <div ref={bottomRef} />

      <Box
        sx={{
          position: "fixed",
          bottom: 90,
          left: 0,
          right: 0,
        }}
      >
        <BaseCommentBar
          value={comment}
          onChange={setComment}
          onSend={handleSend}
          loading={isPending}
          disabled={!data}
        />
      </Box>
    </>
  );
}
