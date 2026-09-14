"""Check shipped archive documentation links resolve or point at the repository."""

import pathlib
import re
import sys

GITHUB_BASE = "https://github.com/Monska85/hostlens/"


def bad_links(roots):
    broken = []
    for root in roots:
        base = pathlib.Path(root)
        for path in sorted(base.rglob("*.md")):
            text = path.read_text()
            for target in re.findall(r"\]\(([^)\s]+)\)", text):
                if target.startswith(("http://", "https://")):
                    if not target.startswith(GITHUB_BASE):
                        broken.append(f"{path.relative_to(base)}: foreign URL {target}")
                elif target.startswith("/"):
                    broken.append(f"{path.relative_to(base)}: absolute target {target}")
                elif not (path.parent / target.split("#", 1)[0]).exists():
                    broken.append(f"{path.relative_to(base)}: unresolved {target}")
    return broken


if __name__ == "__main__":
    broken = bad_links(sys.argv[1:])
    if broken:
        print("Broken documentation links:", file=sys.stderr)
        print("\n".join(broken), file=sys.stderr)
        sys.exit(1)
    print("All shipped documentation links resolve.")
