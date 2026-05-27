import { useState } from "react";

import { useCase, useComments } from "@features/detail/hooks";

import { CommentBar as BaseCommentBar } from "@shared/components/core";

export function CommentBar() {
  const { data } = useCase();
  const [comment, setComment] = useState("");
  const { submit, isPending } = useComments();

  const handleSend = () => {
    submit(comment);
    setComment("");
  };

  return (
    <BaseCommentBar value={comment} onChange={setComment} onSend={handleSend} loading={isPending} disabled={!data} />
  );
}
