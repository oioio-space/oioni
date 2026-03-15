---
name: use-beads-always
description: Always use beads (bd) for task/issue tracking — never TodoWrite or TaskCreate
type: feedback
---

Use beads (`bd create`, `bd update`, `bd close`) for ALL task tracking as soon as possible.

**Why:** User preference — beads is the project's persistent tracking system with memory and dependencies. TodoWrite/TaskCreate are transient and don't persist across sessions.

**How to apply:** Before writing any code, create a beads issue. When starting work, `bd update --status=in_progress`. When done, `bd close`. Never use TodoWrite or TaskCreate for project tasks.
