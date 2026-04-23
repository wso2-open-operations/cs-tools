// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.  See the License for the specific language governing
// permissions and limitations under the License.

import { render, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LexicalComposer } from "@lexical/react/LexicalComposer";
import { RichTextPlugin } from "@lexical/react/LexicalRichTextPlugin";
import { ContentEditable } from "@lexical/react/LexicalContentEditable";
import { LexicalErrorBoundary } from "@lexical/react/LexicalErrorBoundary";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { useEffect } from "react";
import {
  ImageNode,
  $createImageNode,
  type SerializedImageNode,
} from "@components/rich-text-editor/ImageNode";
import { LinkNode } from "@lexical/link";
import { HeadingNode, QuoteNode } from "@lexical/rich-text";
import { CodeNode } from "@lexical/code";
import { ListItemNode, ListNode } from "@lexical/list";

const initialConfig = {
  namespace: "ImageNodeTest",
  nodes: [ListNode, ListItemNode, ImageNode, CodeNode, LinkNode, HeadingNode, QuoteNode],
  onError: () => {},
  editable: true,
};

function TestHarness({
  runTest,
}: {
  runTest: (result: { passed: boolean }) => void;
}) {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    editor.update(() => {
      const node = $createImageNode("https://example.com/img.png", "Alt text");
      const cloned = ImageNode.clone(node);
      const json = node.exportJSON() as SerializedImageNode;
      const serialized: SerializedImageNode = {
        type: "image",
        version: 1,
        src: "https://imported.com/img.png",
        altText: "Imported alt",
      };
      const imported = ImageNode.importJSON(serialized);

      const passed =
        node instanceof ImageNode &&
        cloned !== node &&
        cloned instanceof ImageNode &&
        (cloned as ImageNode & { __src: string }).__src === "https://example.com/img.png" &&
        json.type === "image" &&
        json.src === "https://example.com/img.png" &&
        json.altText === "Alt text" &&
        imported instanceof ImageNode &&
        (imported as ImageNode & { __src: string }).__src === "https://imported.com/img.png" &&
        (imported as ImageNode & { __altText: string }).__altText === "Imported alt";

      runTest({ passed });
    });
  }, [editor, runTest]);

  return <div data-testid="harness" />;
}

describe("ImageNode", () => {
  it("getType returns 'image'", () => {
    expect(ImageNode.getType()).toBe("image");
  });

  it("$createImageNode, clone, exportJSON, importJSON work within editor", async () => {
    const results: { passed: boolean }[] = [];
    render(
      <LexicalComposer initialConfig={initialConfig}>
        <RichTextPlugin
          contentEditable={<ContentEditable data-testid="editable" />}
          placeholder={null}
          ErrorBoundary={LexicalErrorBoundary}
        />
        <TestHarness runTest={(r) => results.push(r)} />
      </LexicalComposer>,
    );
    await waitFor(() => expect(results.length).toBeGreaterThan(0));
    expect(results[0]?.passed).toBe(true);
  });
});
