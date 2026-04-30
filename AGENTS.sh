#!/bin/bash

# This file is intended to be integrated into Claude code, opencode, etc as a session start hook.

echo '<context file="AGENTS.sh">'

hooksh entrypoints --format call-tree --limit 20 --depth 2 --functions --exported-only

hooksh packages --kind go-package-doc --limit 10

hooksh docs --kind md --limit 10

echo '</context>'
