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
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { cases } from "@features/cases/api/cases.queries";

export function useDetailComments(id: string) {
  const queryClient = useQueryClient();
  const [comment, setComment] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);

  const { data: comments, isFetching: isCommentsRefetching } = useQuery({
    ...cases.comments(id),
    select: (data) => [...data].sort((a, b) => a.createdOn.getTime() - b.createdOn.getTime()),
  });

  const mutation = useMutation({
    ...cases.createComment(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: cases.comments(id).queryKey });
      setComment("");
    },
  });

  const isSendingComment = mutation.status !== "idle" && mutation.isPending && isCommentsRefetching;

  const handleSend = () => {
    if (!comment.trim()) return;
    mutation.mutate({ content: comment, type: "comments" });
  };

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [comments]);

  return { comments, comment, setComment, handleSend, isSendingComment, bottomRef };
}
