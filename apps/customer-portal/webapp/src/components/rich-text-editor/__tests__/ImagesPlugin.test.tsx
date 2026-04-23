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

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LexicalComposer } from "@lexical/react/LexicalComposer";
import { RichTextPlugin } from "@lexical/react/LexicalRichTextPlugin";
import { ContentEditable } from "@lexical/react/LexicalContentEditable";
import { LexicalErrorBoundary } from "@lexical/react/LexicalErrorBoundary";
import ImagesPlugin from "@components/rich-text-editor/ImagesPlugin";
import { ImageNode } from "@components/rich-text-editor/ImageNode";
import { LinkNode } from "@lexical/link";
import { HeadingNode, QuoteNode } from "@lexical/rich-text";
import { CodeNode } from "@lexical/code";
import { ListItemNode, ListNode } from "@lexical/list";

const initialConfig = {
  namespace: "ImagesPluginTest",
  nodes: [ListNode, ListItemNode, ImageNode, CodeNode, LinkNode, HeadingNode, QuoteNode],
  onError: () => {},
  editable: true,
};

function renderWithLexical() {
  return render(
    <LexicalComposer initialConfig={initialConfig}>
      <RichTextPlugin
        contentEditable={<ContentEditable data-testid="editable" />}
        placeholder={null}
        ErrorBoundary={LexicalErrorBoundary}
      />
      <ImagesPlugin />
    </LexicalComposer>,
  );
}

describe("ImagesPlugin", () => {
  it("renders without throwing", () => {
    const { getByTestId } = renderWithLexical();
    expect(getByTestId("editable")).toBeInTheDocument();
  });

});
