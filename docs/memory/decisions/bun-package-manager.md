# Decision: Switched web/ package manager to bun

> Engram #108 | 2026-04-06 | topic: `config/web-package-manager`

**What**: Replaced npm with bun as package manager for web/ (Astro frontend).

**Why**: User preference — bun is faster and preferred.

**Where**: `web/package.json` — added `"packageManager": "bun@1.3.11"`, removed package-lock.json, bun.lockb generated.

**Learned**: Always use `bun install`, `bun add`, `bun run build` for the web/ directory. No npm commands.
