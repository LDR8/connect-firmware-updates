#!/bin/sh
# Puts the GitHub owner of this repository into every place the extension id
# is derived from or written: go.mod, the fault domain, the generated client,
# the licence and the codeowner. Run it once, before the first release, then
# `make client && make check` and commit. The id never changes after the first
# release.
#
#   sh scripts/set-owner.sh <github-user-or-org>
set -e

OWNER=$(printf '%s' "$1" | tr 'A-Z' 'a-z')
if [ -z "$OWNER" ]; then
	echo "usage: sh scripts/set-owner.sh <github-user-or-org>"
	exit 1
fi

cd "$(dirname "$0")/.."
OLD=$(sed -n 's|^module github\.com/\([^/]*\)/.*|\1|p' go.mod)

for f in go.mod faults.go main.go LICENSE module.yaml client/package.json client/README.md \
	client/src/index.ts client/src/generated/operations.ts; do
	[ -f "$f" ] || continue
	sed -i \
		-e "s|github.com/$OLD/|github.com/$OWNER/|g" \
		-e "s|com.github.aux.$OLD.|com.github.aux.$OWNER.|g" \
		-e "s|(c) 2026 $OLD|(c) 2026 $OWNER|" \
		-e "s|^# codeowner: $OLD|codeowner: $OWNER|" \
		"$f"
done

echo "The owner is now $OWNER. Next: make client && make check, then commit."
