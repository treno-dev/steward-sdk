"""Generates the contract's Python messages and gRPC services into src/steward_plugin/_gen.

The generated code is not committed. Run it before building or testing the package:

    python scripts/generate.py
"""

import shutil
import sys
from importlib.resources import files
from pathlib import Path

from grpc_tools import protoc

ROOT = Path(__file__).resolve().parents[1]
PROTO = ROOT.parents[1] / "proto"
OUT = ROOT / "src" / "steward_plugin" / "_gen"
CONTRACT = "steward/plugin/v1/plugin.proto"


def main() -> int:
    shutil.rmtree(OUT, ignore_errors=True)
    OUT.mkdir(parents=True)

    status = protoc.main(
        [
            "protoc",
            f"-I{PROTO}",
            f"-I{files('grpc_tools') / '_proto'}",
            f"--python_out={OUT}",
            f"--pyi_out={OUT}",
            f"--grpc_python_out={OUT}",
            CONTRACT,
        ]
    )

    if status != 0:
        return status

    for folder in [OUT, *OUT.rglob("*")]:
        if folder.is_dir():
            (folder / "__init__.py").touch()

    # protoc imports its own output by the proto path; the package keeps it under _gen instead.
    grpc_module = OUT / "steward" / "plugin" / "v1" / "plugin_pb2_grpc.py"
    source = grpc_module.read_text()
    grpc_module.write_text(
        source.replace(
            "from steward.plugin.v1 import plugin_pb2",
            "from steward_plugin._gen.steward.plugin.v1 import plugin_pb2",
        )
    )

    return 0


if __name__ == "__main__":
    sys.exit(main())
