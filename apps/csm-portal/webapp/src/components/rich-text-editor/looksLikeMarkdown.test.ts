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
import { looksLikeMarkdown } from "./looksLikeMarkdown";

const CHATGPT_ANSWER = `Here's how to fix it:

## Steps

1. Open the **Settings** page
2. Click *Advanced*
3. Restart the service

\`\`\`bash
sudo systemctl restart wso2am
\`\`\`

Let me know if that helps!`;

const GITHUB_ISSUE = `### Describe the bug
The gateway returns 500 when the token is expired.

### Steps to reproduce
- Call the API with an expired token
- Observe the response

Related to #123.`;

const SERVICENOW_NOTE = `Customer reported the API manager is slow since the upgrade.
Checked the logs, nothing obvious. Waiting for a heap dump.
Ref: INC0012345, contact john.doe@example.com`;

const JAVA_STACK_TRACE = `java.lang.NullPointerException: Cannot invoke "String.length()" because "s" is null
\tat org.wso2.carbon.Foo.bar(Foo.java:42)
\tat org.wso2.carbon.Baz.run(Baz.java:17)
Caused by: java.io.IOException: Connection reset
\t... 12 more`;

const LOG_EXCERPT = `[2026-05-01 10:15:32,123]  INFO - Server started on port 9443
[2026-05-01 10:15:33,001]  WARN - Slow query *took* 1200ms
[2026-05-01 10:15:34,500] ERROR - Failed to load config_file__v2 from C:\\wso2\\conf_dir\\deployment.toml`;

const HYPHEN_PROSE = `The deployment was well-known to be flaky.
-rated by the customer as high priority, and
low-risk according to the team.`;

describe("looksLikeMarkdown", () => {
  it.each([
    ["ChatGPT-style answer", CHATGPT_ANSWER],
    ["GitHub issue body", GITHUB_ISSUE],
    ["fenced block, tildes", "~~~\ncode\n~~~"],
    ["fenced block with a language", "```js\nconst a = 1;\n```"],
    ["ATX heading", "# Title\nsome text"],
    ["h6 heading", "###### Tiny"],
    ["two dash list lines", "- one\n- two"],
    ["star list lines", "* one\n* two"],
    ["plus list lines", "+ one\n+ two"],
    ["ordered list with dots", "1. one\n2. two"],
    ["ordered list with parens", "1) one\n2) two"],
    ["blockquote", "> quoted text"],
    ["GFM table", "| a | b |\n|---|---|\n| 1 | 2 |"],
    ["GFM table without outer pipes", "a | b\n--- | ---\n1 | 2"],
    ["star bold", "this is **important** stuff"],
    ["underscore bold", "this is __really important__ stuff"],
    ["absolute link", "see [the docs](https://example.com/docs)"],
    ["relative link", "see [the guide](./guide/intro.md)"],
    ["strikethrough", "this is ~~wrong~~ right"],
    ["a list after an intro line", "Steps:\n1. open\n2. click"],
    ["list items separated by a blank line", "- a\n\n- b"],
    ["an indented (YAML-style) list", "items:\n  - a\n  - b"],
    ["an e-mail quote", "> On Monday Bob wrote:\n> hi"],
    ["CRLF lists", "- a\r\n- b\r\n"],
    ["CRLF heading", "# H\r\ntext"],
    ["CRLF table", "| a | b |\r\n|---|---|\r\n| 1 | 2 |"],
    ["CRLF fence", "```\r\ncode\r\n```"],
    ["a mailto link", "[mail me](mailto:a@b.com)"],
    ["an anchor link", "[top](#intro)"],
    ["a root-relative link", "[docs](/docs/intro)"],
    ["an image badge inside a link", "[![alt](https://i.com/a.png)](https://x.com)"],
    ["a '# comment' line (accepted: it is an ATX heading)", "# install deps\nnpm i"],
    ["a list in an e-mail signature", "Thanks,\nBob\n-- \nBob Smith\n- Support Eng\n- WSO2"],
  ])("detects %s", (_name, text) => {
    expect(looksLikeMarkdown(text)).toBe(true);
  });

  it.each([
    ["empty", ""],
    ["whitespace", "  \n "],
    ["ServiceNow-style plain note", SERVICENOW_NOTE],
    ["Java stack trace", JAVA_STACK_TRACE],
    ["log excerpt", LOG_EXCERPT],
    ["prose with a hyphenated word at a line start", HYPHEN_PROSE],
    ["a lone *italic*", "this is *emphasised* only"],
    ["a single backtick span", "run `ls -la` to list"],
    ["an issue reference", "see #123 for details"],
    ["a Windows path with underscores", "C:\\Program Files\\my_app\\some_dir\\run_me.exe"],
    ["a Python dunder", "def __init__(self): pass"],
    ["a single numbered line", "1. restart the server"],
    ["a single list line", "- only one"],
    ["an e-mail address", "write to first_last@example.com"],
    ["a bare URL", "https://example.com/a_b/c"],
    ["a glob", "src/**/*.ts and lib/**/*.js"],
    ["C pointers", "int **a, **b;"],
    ["a hash without a space", "#hashtag and #123"],
    ["bracket then paren in code", "callbacks[0](value)"],
    ["a shell prompt with an arrow", "C:\\> dir"],
    ["two list lines separated by prose", "- a\nsome prose\n- b"],
    ["a lone list line in a signature", "Thanks,\nBob\n-- \nBob Smith\n- Support Eng"],
    ["a numeric power", "2**3**4"],
    ["a glob with **", "rm **/*.js"],
    ["an indexed call with a path", "foo[0](x/y)"],
    ["bracket text", "see [1](2)"],
    ["a bare relative path link", "[x](foo/bar)"],
    ["a setext heading", "Title\n=====\ntext"],
    ["indented code", "    code line\n    other"],
    ["a Python dunder in code", "def __init__(self):\n    x = 1"],
    ["a stack trace", "at Foo.bar (/a/b.js:1:2)\n  at x"],
    ["an issue reference", "#123 broke"],
  ])("does not treat %s as Markdown", (_name, text) => {
    expect(looksLikeMarkdown(text)).toBe(false);
  });
});

describe("looksLikeMarkdown stays fast on adversarial input", () => {
  const adversarial: Record<string, string> = {
    stars: "*".repeat(200_000),
    "open bold": "**a ".repeat(50_000),
    "open bold, one word": "**" + "a".repeat(200_000),
    "long open bold": ("**x" + "y".repeat(50)).repeat(4000),
    underscores: "_".repeat(200_000),
    "open underscore bold": "__a ".repeat(50_000),
    "underscore bold on one line": "__" + "a b ".repeat(15_000),
    "open brackets": "[".repeat(60_000),
    "bracket text": "[a".repeat(30_000),
    "bracket, paren": "[a](".repeat(50_000),
    "long link target": "[a](https://" + "x".repeat(200_000),
    pipes: "|".repeat(200_000),
    "pipes lines": "a|b\n" + "|".repeat(100_000) + "\n",
    "table separator": "a|b\n" + "|---".repeat(50_000) + "x\n",
    tildes: "~".repeat(200_000),
    "open strike": "~~a ".repeat(50_000),
    "long strike": "~~" + "a".repeat(200_000),
    spaces: " ".repeat(200_000) + "x",
    "hash spaces": "#" + " ".repeat(200_000),
    backticks: "`".repeat(200_000),
    "fence spaces": "```" + " ".repeat(200_000) + "x",
    "quote spaces": ">" + " ".repeat(200_000),
    "big plain text": "hello world line\n".repeat(12_000),
    "big list": "- item\n".repeat(30_000),
  };

  it.each(Object.entries(adversarial))("%s", (_name, text) => {
    const start = performance.now();
    looksLikeMarkdown(text);
    expect(performance.now() - start).toBeLessThan(200);
  });

  it("judges a long paste on its start", () => {
    expect(looksLikeMarkdown("# Title\n" + "x".repeat(100_000))).toBe(true);
    expect(looksLikeMarkdown("x".repeat(30_000) + "\n# Late heading")).toBe(false);
  });
});
