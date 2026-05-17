---
description: Release glowed using the project release skill
argument-hint: "[version]"
---
Use the project skill `.agents/skills/release/SKILL.md` and run the glowed release workflow end-to-end.

Version argument, if provided: `$1`

If no version is provided, determine the next release version according to the skill instructions.

Before tagging or publishing, summarize the planned version, current git state, and release steps. Then proceed if safe.
