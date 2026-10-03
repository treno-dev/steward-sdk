"""Generates the contract's Python messages and gRPC services into src/steward_sdk/_gen, with buf.

The generated code is not committed. Run it before building or testing the package:

    python scripts/generate.py
"""

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "src" / "steward_sdk" / "_gen"


def main() -> int:
    status = subprocess.run(["buf", "generate"], cwd=ROOT).returncode

    if status != 0:
        return status

    for folder in [OUT, *OUT.rglob("*")]:
        if folder.is_dir():
            (folder / "__init__.py").touch()

    # The plugin imports its own output by the proto path; the package keeps it under _gen instead.
    grpc_module = OUT / "steward" / "plugin" / "v1" / "plugin_pb2_grpc.py"
    source = grpc_module.read_text()
    grpc_module.write_text(
        source.replace(
            "from steward.plugin.v1 import plugin_pb2",
            "from steward_sdk._gen.steward.plugin.v1 import plugin_pb2",
        )
    )

    return 0


if __name__ == "__main__":
    sys.exit(main())
