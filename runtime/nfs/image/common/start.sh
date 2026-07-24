#!/bin/bash
set -euo pipefail

/usr/bin/update-control-plane-ca
mount --rbind /host/dev /dev
mount --make-rslave /dev
exec "$@"
