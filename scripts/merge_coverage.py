#!/usr/bin/env python3
"""Une perfiles Go mode=set por bloque para contar cobertura entre procesos."""

import argparse
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path)
    parser.add_argument("profiles", type=Path, nargs="+")
    args = parser.parse_args()

    blocks: dict[tuple[str, str], int] = {}
    for profile in args.profiles:
        with profile.open(encoding="utf-8") as source:
            if source.readline().strip() != "mode: set":
                parser.error(f"{profile}: se esperaba mode: set")
            for line in source:
                location, statements, count = line.split()
                key = (location, statements)
                blocks[key] = max(blocks.get(key, 0), int(count))

    with args.output.open("w", encoding="utf-8") as target:
        target.write("mode: set\n")
        for (location, statements), count in sorted(blocks.items()):
            target.write(f"{location} {statements} {count}\n")


if __name__ == "__main__":
    main()
