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
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

"""Regenerate ballerina_response_fields.json from a digiops-cs checkout.

The Ballerina entity-service lives in a different repository and is being
decommissioned, so parity_test.go compares against a frozen inventory rather
than reading that source at test time. This script produces that inventory.

Usage:
    python3 extract_ballerina_fields.py <path-to-digiops-cs/entity-service> \\
        > ballerina_response_fields.json

The checkout path is a required argument. It differs per machine, so a default
would both fail for everyone else and bake one contributor's local directory
layout into the repository.
"""

import json
import os
import re
import subprocess
import sys

TYPES_REL = "modules/servicenow/types.bal"

# Request-side types are excluded. A filter this service has not ported is an
# unimplemented feature, not silent data loss on a response — a different and
# far less dangerous class.
REQUEST_SUFFIXES = ("Payload", "Request", "Filters", "Pagination", "Config", "RetryConfig")

# Matches a record field declaration, tolerating optional markers, arrays,
# unions, defaults and quoted identifiers ('type, 'limit).
FIELD = re.compile(r"^\s*[\w\.\|\?\[\]<>,\s]*?\s'?(\w+)\s*\??\s*(=[^;]*)?;\s*$")


def response_types(types_path):
    """Return {typeName: [fieldName, ...]} for response-side records."""
    out, current, depth = {}, None, 0
    with open(types_path, encoding="utf-8") as handle:
        for line in handle:
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            opener = re.match(r"public type (\w+) record \{\|?", stripped)
            if opener and depth == 0:
                current = opener.group(1)
                out.setdefault(current, set())
                depth = 1
                continue
            if current is None:
                continue
            depth += stripped.count("{") - stripped.count("}")
            if depth <= 0:
                current = None
                continue
            if "..." in stripped:                       # rest field (json...;)
                continue
            if stripped.startswith("record") or stripped.endswith("{|"):
                continue                                # nested opener; its fields follow
            match = FIELD.match(line)
            if match:
                out[current].add(match.group(1))
    return {name: sorted(fields) for name, fields in out.items()
            if fields and not name.endswith(REQUEST_SUFFIXES)}


def main():
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    root = sys.argv[1]
    types_path = os.path.join(root, TYPES_REL)
    if not os.path.isfile(types_path):
        sys.exit(f"not found: {types_path}\nExpected a digiops-cs/entity-service checkout.")

    types = response_types(types_path)
    fields = sorted({f for fs in types.values() for f in fs})
    revision = subprocess.run(
        ["git", "-C", root, "rev-parse", "--short", "HEAD"],
        capture_output=True, text=True, check=False,
    ).stdout.strip() or "unknown"

    json.dump({
        "_comment": "Frozen inventory of response-side field names from the Ballerina "
                    "entity-service. Regenerate with extract_ballerina_fields.py; "
                    "see parity_test.go for how it is used.",
        "source": f"digiops-cs/entity-service/{TYPES_REL}",
        "sourceRevision": revision,
        "responseTypes": len(types),
        "fields": fields,
    }, sys.stdout, indent=2)
    sys.stdout.write("\n")
    print(f"{len(types)} response types, {len(fields)} distinct field names", file=sys.stderr)


if __name__ == "__main__":
    main()
