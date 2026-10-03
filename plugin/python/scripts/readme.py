"""Replaces each marked block in README.md with the file at the path in its marker, so an example is
written once and the README never drifts from what `steward plugin init` generates. Paths are
relative to the SDK package, and placeholders in a template get sample values.

A block looks like this, and everything between the code fences is replaced:

    <!-- template: ../../templates/python/src/plugin.py.tmpl -->
    ```python
    ```
    <!-- /template -->

    python scripts/readme.py           rewrite README.md
    python scripts/readme.py --check   exit 1 if README.md is out of date
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
README = ROOT / "README.md"

# The values the CLI fills in when it renders a template.
VALUES = {"Name": "my-plugin", "Title": "My plugin"}

PLACEHOLDER = re.compile(r"\[\[\.(\w+)\]\]")
BLOCK = re.compile(r"(<!-- template: (\S+) -->\n```\w*\n).*?(```\n<!-- /template -->)", re.DOTALL)


def render(text: str) -> str:
    return PLACEHOLDER.sub(lambda found: VALUES.get(found.group(1), found.group(0)), text)


def fill(found: re.Match[str]) -> str:
    open_, path, close = found.groups()
    source = render((ROOT / path).read_text())

    return f"{open_}{source if source.endswith(chr(10)) else source + chr(10)}{close}"


def main() -> int:
    current = README.read_text()
    updated = BLOCK.sub(fill, current)

    if "--check" in sys.argv:
        if updated != current:
            print("README.md is out of date: run `python scripts/readme.py`.", file=sys.stderr)
            return 1

        return 0

    README.write_text(updated)

    return 0


if __name__ == "__main__":
    sys.exit(main())
