#!/usr/bin/env python3
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.

"""Checks every OpenAPI spec in the repository for YAML mistakes that parse
without an error but change what the spec says.

1. A sentence that became a key. In a flow mapping an unquoted comma ends the
   value, so

       teamKey: { type: string, description: The team, where there is one. }

   gives "description: The team" plus a key "where there is one." with no
   value, and the spec fails validation where it is deployed (#2605). Quote
   the description.
2. A key written twice in one mapping. YAML keeps the last one and drops the
   first without a word.
3. A file that does not parse at all.

Usage: check-openapi-specs.py [file ...]  (default: every tracked
*openapi*.yaml / *.yml). Exits 1 when anything is found.
"""

import subprocess
import sys

import yaml


def tracked_specs():
    out = subprocess.run(["git", "ls-files", "*openapi*.yaml", "*openapi*.yml"],
                         check=True, capture_output=True, text=True).stdout
    return [p for p in out.splitlines() if p]


def is_implicit_null(node):
    """A value left empty: "key:" or a dangling flow-mapping entry."""
    return isinstance(node, yaml.ScalarNode) and node.tag == "tag:yaml.org,2002:null" and node.value in ("", "~")


def check_node(node, path, problems):
    if isinstance(node, yaml.MappingNode):
        seen = {}
        for key, value in node.value:
            if not isinstance(key, yaml.ScalarNode):
                check_node(value, path, problems)
                continue
            line = key.start_mark.line + 1
            if key.value in seen and key.value != "<<":
                problems.append((line, f"'{key.value}' is written twice in this mapping "
                                       f"(first on line {seen[key.value]}); YAML keeps only the last"))
            seen.setdefault(key.value, line)
            if " " in key.value.strip() and is_implicit_null(value):
                problems.append((line, f"'{key.value}' became a key with no value: an unquoted comma "
                                       "probably split a description; quote the whole value"))
            check_node(value, path + [key.value], problems)
    elif isinstance(node, yaml.SequenceNode):
        for item in node.value:
            check_node(item, path, problems)


def check_file(path):
    try:
        with open(path, encoding="utf-8") as f:
            root = yaml.compose(f, Loader=yaml.SafeLoader)
    except yaml.YAMLError as e:
        return [(getattr(getattr(e, "problem_mark", None), "line", -1) + 1, f"does not parse: {e}")]
    problems = []
    if root is not None:
        check_node(root, [], problems)
    return problems


def main(argv):
    files = argv[1:] or tracked_specs()
    failed = 0
    for path in files:
        for line, msg in sorted(check_file(path)):
            print(f"{path}:{line}: {msg}")
            failed += 1
    print(f"checked {len(files)} OpenAPI spec(s): {'no problems' if not failed else f'{failed} problem(s)'}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
