#!/usr/bin/env bash
set -euo pipefail

perl -pi -e 's/[ \t]+$//' \
  frontend/wailsjs/go/app/App.d.ts \
  frontend/wailsjs/go/app/App.js \
  frontend/wailsjs/go/models.ts

# Wails currently emits two stable whitespace-only diffs in models.ts:
# one blank line before the first app namespace and one extra blank line at EOF.
perl -0pi -e 's/\n}\n\nexport namespace app \{/\n}\nexport namespace app {/g; s/\n}\n+\z/\n}\n/' \
  frontend/wailsjs/go/models.ts
