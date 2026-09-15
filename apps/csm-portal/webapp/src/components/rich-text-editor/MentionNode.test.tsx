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

import { describe, expect, it } from "vitest";
import { $createParagraphNode, $getRoot, $insertNodes, createEditor } from "lexical";
import { $generateHtmlFromNodes, $generateNodesFromDOM } from "@lexical/html";
import {
  $createMentionNode,
  $isMentionNode,
  MentionNode,
} from "./MentionNode";

/** Minimal headless editor registering only what MentionNode needs. */
function createTestEditor() {
  return createEditor({
    namespace: "MentionNodeTest",
    nodes: [MentionNode],
    onError: (error) => {
      throw error;
    },
  });
}

describe("MentionNode", () => {
  it("exportJSON/importJSON round-trips the mention name and user id", () => {
    const editor = createTestEditor();

    editor.update(
      () => {
        const root = $getRoot();
        root.clear();
        const mention = $createMentionNode("Jane Doe", "user-123");
        $insertNodes([mention]);
      },
      { discrete: true },
    );
    const serialized = editor.getEditorState().toJSON();

    const editor2 = createTestEditor();
    editor2.setEditorState(editor2.parseEditorState(serialized));

    editor2.getEditorState().read(() => {
      const root = $getRoot();
      const mention = root.getFirstDescendant();
      expect($isMentionNode(mention)).toBe(true);
      if ($isMentionNode(mention)) {
        expect(mention.__mentionName).toBe("Jane Doe");
        expect(mention.__userId).toBe("user-123");
        expect(mention.getTextContent()).toBe("@Jane Doe");
      }
    });
  });

  it("exportDOM renders a span carrying data-mention-user-id and the @name text", () => {
    const editor = createTestEditor();

    editor.update(
      () => {
        const root = $getRoot();
        root.clear();
        const mention = $createMentionNode("Jane Doe", "user-123");
        $insertNodes([mention]);
      },
      { discrete: true },
    );

    let html = "";
    editor.getEditorState().read(() => {
      html = $generateHtmlFromNodes(editor);
    });

    expect(html).toContain('data-mention-user-id="user-123"');
    expect(html).toContain("@Jane Doe");
  });

  it("importDOM converts a data-mention-user-id span back into a MentionNode", () => {
    const editor = createTestEditor();
    const dom = new DOMParser().parseFromString(
      '<span data-mention-user-id="user-456">@Jane Smith</span>',
      "text/html",
    );

    editor.update(
      () => {
        const root = $getRoot();
        root.clear();
        const nodes = $generateNodesFromDOM(editor, dom);
        const paragraph = $createParagraphNode();
        paragraph.append(...nodes);
        root.append(paragraph);
      },
      { discrete: true },
    );

    editor.getEditorState().read(() => {
      const root = $getRoot();
      const mention = root.getFirstDescendant();
      expect($isMentionNode(mention)).toBe(true);
      if ($isMentionNode(mention)) {
        expect(mention.__userId).toBe("user-456");
        expect(mention.__mentionName).toBe("Jane Smith");
      }
    });
  });

  it("does not treat a plain span without the mention attribute as a mention", () => {
    const editor = createTestEditor();
    const dom = new DOMParser().parseFromString(
      "<span>just text</span>",
      "text/html",
    );

    editor.update(
      () => {
        const root = $getRoot();
        root.clear();
        const nodes = $generateNodesFromDOM(editor, dom);
        const paragraph = $createParagraphNode();
        paragraph.append(...nodes);
        root.append(paragraph);
      },
      { discrete: true },
    );

    editor.getEditorState().read(() => {
      const root = $getRoot();
      const mention = root.getFirstDescendant();
      expect($isMentionNode(mention)).toBe(false);
    });
  });
});
