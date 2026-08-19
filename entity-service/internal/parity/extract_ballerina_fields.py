import re, sys, json, subprocess, os

SRC = "/Users/sasmitha/chat_bot/novera/digiops-cs/entity-service/modules/servicenow/types.bal"

# Request-side types are excluded: a missing filter is an unported feature, not
# silent data loss on a response. Different, lower-severity class.
REQUEST_SUFFIXES = ("Payload", "Request", "Filters", "Pagination", "Config", "RetryConfig")

FIELD = re.compile(r"^\s*[\w\.\|\?\[\]<>,\s]*?\s'?(\w+)\s*\??\s*(=[^;]*)?;\s*$")

def types_in(path):
    text = open(path).read().split("\n")
    out, cur, depth = {}, None, 0
    for line in text:
        stripped = line.strip()
        if stripped.startswith("#") or not stripped:
            continue
        m = re.match(r"public type (\w+) record \{\|?", stripped)
        if m and depth == 0:
            cur = m.group(1); out.setdefault(cur, set()); depth = 1; continue
        if cur is None:
            continue
        depth += stripped.count("{") - stripped.count("}")
        if depth <= 0:
            cur = None; continue
        if "..." in stripped:          # rest field (json...;)
            continue
        if stripped.startswith("record") or stripped.endswith("{|"):
            continue                    # nested record opener; its fields follow
        fm = FIELD.match(line)
        if fm:
            out[cur].add(fm.group(1))
    return out

allt = types_in(SRC)
resp = {k: sorted(v) for k, v in allt.items()
        if not k.endswith(REQUEST_SUFFIXES) and v}

flat = sorted({f for fs in resp.values() for f in fs})
rev = subprocess.run(["git","-C",os.path.dirname(SRC),"rev-parse","--short","HEAD"],
                     capture_output=True, text=True).stdout.strip() or "unknown"
print(json.dumps({
  "_comment": "Frozen inventory of response-side field names from the Ballerina entity-service. See parity_test.go.",
  "source": "digiops-cs/entity-service/modules/servicenow/types.bal",
  "sourceRevision": rev,
  "responseTypes": len(resp),
  "fields": flat,
}, indent=2))
print(f"\n-- {len(resp)} response types, {len(flat)} distinct field names", file=sys.stderr)
