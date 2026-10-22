#!/bin/sh
set -eu
backup_dir=/var/lib/mythra-licensing/backups
mkdir -p "$backup_dir"
/opt/mythra-licensing/license-admin -action backup -output "$backup_dir/$(date -u +%Y%m%dT%H%M%SZ).sqlite"
find "$backup_dir" -type f -name '*.sqlite' -mtime +7 -delete
