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

import {
  TextNode,
  type DOMConversionMap,
  type DOMConversionOutput,
  type EditorConfig,
  type LexicalNode,
  type NodeKey,
  type SerializedTextNode,
  type Spread,
} from "lexical";

/** Class applied to a mention's DOM node — themed in Editor.tsx alongside `.editor-link`. */
export const MENTION_CLASS_NAME = "editor-mention";

export type SerializedMentionNode = Spread<
  {
    mentionName: string;
    userId: string;
  },
  SerializedTextNode
>;

function convertMentionElement(domNode: HTMLElement): DOMConversionOutput | null {
  const userId = domNode.getAttribute("data-mention-user-id");
  if (userId === null) return null;
  const textContent = domNode.textContent ?? "";
  const name = textContent.startsWith("@") ? textContent.slice(1) : textContent;
  return { node: $createMentionNode(name, userId) };
}

/**
 * An atomic, non-editable inline mention (`@Some Name`) carrying the
 * mentioned user's id. A `TextNode` subclass (not a `DecoratorNode` like
 * {@link ImageNode}) so it behaves like text for cursor movement, selection,
 * and copy/paste, while still being a single, indivisible unit — see
 * `isTextEntity`/segmented mode below.
 *
 * Round-trips through `$generateHtmlFromNodes`/`$generateNodesFromDOM` (the
 * same HTML-conversion path `Editor.tsx` already uses for the rest of the
 * document) as `<span data-mention-user-id="{userId}">@{mentionName}</span>`,
 * which is the shape `CsmCaseCommentInput` pulls `mentionedUserIds` back out
 * of on submit.
 */
export class MentionNode extends TextNode {
  __mentionName: string;
  __userId: string;

  static getType(): string {
    return "mention";
  }

  static clone(node: MentionNode): MentionNode {
    return new MentionNode(node.__mentionName, node.__userId, node.__text, node.__key);
  }

  static importJSON(serializedNode: SerializedMentionNode): MentionNode {
    const node = $createMentionNode(serializedNode.mentionName, serializedNode.userId);
    node.setTextContent(serializedNode.text);
    node.setFormat(serializedNode.format);
    node.setDetail(serializedNode.detail);
    node.setMode(serializedNode.mode);
    node.setStyle(serializedNode.style);
    return node;
  }

  constructor(mentionName: string, userId: string, text?: string, key?: NodeKey) {
    super(text ?? `@${mentionName}`, key);
    this.__mentionName = mentionName;
    this.__userId = userId;
  }

  exportJSON(): SerializedMentionNode {
    return {
      ...super.exportJSON(),
      mentionName: this.__mentionName,
      userId: this.__userId,
      type: "mention",
      version: 1,
    };
  }

  createDOM(config: EditorConfig): HTMLElement {
    const dom = super.createDOM(config);
    dom.className = MENTION_CLASS_NAME;
    dom.setAttribute("data-mention-user-id", this.__userId);
    return dom;
  }

  exportDOM(): { element: HTMLElement | null } {
    const element = document.createElement("span");
    element.setAttribute("data-mention-user-id", this.__userId);
    element.className = MENTION_CLASS_NAME;
    element.textContent = `@${this.__mentionName}`;
    return { element };
  }

  static importDOM(): DOMConversionMap<HTMLElement> | null {
    return {
      span: (domNode: HTMLElement) => {
        if (!domNode.hasAttribute("data-mention-user-id")) return null;
        return {
          conversion: convertMentionElement,
          priority: 1,
        };
      },
    };
  }

  // Treated as a single, indivisible unit of text: cannot be entered into or
  // split by typing at its edges, and is deleted/copied as a whole — same
  // behavior the Lexical mentions playground example relies on.
  isTextEntity(): true {
    return true;
  }

  canInsertTextBefore(): boolean {
    return false;
  }

  canInsertTextAfter(): boolean {
    return false;
  }
}

export function $createMentionNode(mentionName: string, userId: string): MentionNode {
  const node = new MentionNode(mentionName, userId);
  node.setMode("segmented");
  return node;
}

export function $isMentionNode(
  node: LexicalNode | null | undefined,
): node is MentionNode {
  return node instanceof MentionNode;
}
