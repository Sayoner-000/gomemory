"""Reproduce R2-02 usando un binario actual y datos temporales; ejecutar desde el repo."""

import os
from pathlib import Path
import subprocess
import tempfile


def main():
    source = Path.cwd()
    if not (source / "go.mod").is_file():
        raise SystemExit("Ejecutar desde la raíz del repositorio gomemory")
    with tempfile.TemporaryDirectory(prefix="gomemory-audit-r2-") as temporary:
        base = Path(temporary)
        binary = base / "mem"
        project = base / "project"
        project.mkdir()
        subprocess.run(
            ["go", "build", "-o", str(binary), "./infrastructure"],
            cwd=source,
            check=True,
        )
        environment = os.environ.copy()
        environment.update(
            GOMEMORY_DATA_HOME=str(base / "data"),
            GOMEMORY_NO_UPDATE_CHECK="1",
            GOMEMORY_NO_CODE_REFRESH="1",
        )
        subprocess.run(["git", "init", "-q"], cwd=project, check=True)
        subprocess.run(
            [str(binary), "init"], cwd=project, env=environment,
            check=True, capture_output=True,
        )
        result = subprocess.run(
            [str(binary), "optimize", "--yes", "--target=interanl"],
            cwd=project, env=environment, capture_output=True, text=True,
        )
        print(f"invalid-target exit={result.returncode}")
        print(result.stdout, end="")
        print(result.stderr, end="")


if __name__ == "__main__":
    main()
