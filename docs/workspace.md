# Workspace layout

`make onboard` / `maven onboard` prepares your config and workspace. Paths below assume the default **`agent.workspace`** of **`~/.maven/workspace`**; change it in **`~/.maven/config.json`** if you prefer another directory (including inside a Git checkout—not recommended unless you intend to track those files).

## After onboard

```text
~/.maven/
  config.json
  workspace/
    AGENTS.md              # system prompt persona (edit to taste)
    SOUL.md                # identity / continuity (edit to taste)
    HEARTBEAT.md           # heartbeat runner task text (starts empty until you edit)
    memory/
      MEMORY.md            # seeded empty; journal + memory store live under memory/
    skills/
      <skill-name>/
        SKILL.md           # optional skills; see Skills in README.md
```

## Skills directory

Default skills root is **`{agent.workspace}/skills`**. Set **`skills.dir`** in config to override.

## Telegram slash commands (optional)

Custom Telegram `/` commands are **`*.md` files** under **`{agent.workspace}/.telegram/slashes/`**, each with YAML frontmatter (`command`, `description`, `type`, …) and a Markdown body used as the prompt. See **`docs/telegram-setup.md`** and the checked-in sample under **`docs/workspace/.telegram/slashes/`**.

## Sample tree in the repo

A copy-paste reference directory lives at **`docs/workspace/`** (see **`docs/workspace/README.md`**). It mirrors onboard defaults plus optional Telegram slashes and one example skill.

Other channel‑specific dirs under **`agent.workspace`** are up to you.
