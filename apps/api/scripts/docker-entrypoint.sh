#!/bin/sh

set -eu

if [ "$#" -eq 0 ]; then
	echo "usage: docker-entrypoint.sh <command> [args...]" >&2
	exit 64
fi

mkdir -p /data "$HOME"
chown -R app:app /data "$HOME"

exec setpriv --reuid=app --regid=app --init-groups "$@"