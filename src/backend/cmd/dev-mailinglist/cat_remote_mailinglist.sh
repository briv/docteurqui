#!/usr/bin/env bash

set -euo pipefail

THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TMP_DIR="$(mktemp -d)"

KEY="$THIS_DIR/../../../../deploying/private.key.secret"
MAILINGLIST="$TMP_DIR/mailinglist"

scp vultr://var/lib/docteurqui/autocontract/mailinglist/mailinglist "$MAILINGLIST"
go run "$THIS_DIR/main.go" \
    -mailinglist-file="$MAILINGLIST" \
    -mailinglist-private-key-file="$KEY"
