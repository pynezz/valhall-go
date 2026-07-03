#!/usr/bin/env bash
# stoker-plugin: AVC denials (24h)
# stoker-order: 81
# stoker-timeout: 30
# stoker-root: yes
#
# Unchanged from stoker v0.1 (Python) — the header format is the
# frozen compatibility boundary between implementations.
set -euo pipefail
if command -v ausearch >/dev/null; then
    ausearch -m AVC,USER_AVC -ts today 2>&1 || true
else
    echo "ausearch not installed (audit package)"
fi
