# Skills

A skill is a YAML-fronted markdown file Maven loads at gateway start (and on every reload). When one of its keywords matches the user's input, the body is injected into the system prompt for that turn.

## Configuration

```json
{
  "skills": {
    "enabled": true,
    "dir": ""
  }
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `true` | Master toggle. |
| `dir` | `""` (means `<workspace>/skills`) | Override the skills directory. |

`maven onboard` creates the default directory.

## File layout

```text
<workspace>/skills/
  writer/
    SKILL.md
  research/
    SKILL.md
```

One folder per skill. Each must contain a `SKILL.md`. Folder name is metadata only — the skill `name` comes from frontmatter.

## SKILL.md format

```markdown
---
name: writer
description: writing helper
keywords: [write, draft, edit]
---

# Writer

You are now in writing mode. Prefer concise, active voice. Avoid filler.
```

| Frontmatter field | Required | Description |
|-------------------|----------|-------------|
| `name` | Yes | Unique skill identifier. Duplicate names across folders → load error. |
| `description` | No | Free text shown by `maven skills info`. |
| `keywords` | No | Lowercased, trimmed, deduplicated, sorted. Empty list ⇒ skill always opt-in via direct activation (no automatic match). |

The body (everything after the second `---`) is the system prompt block. It is **not** processed as markdown — it is sent verbatim to the model.

## CLI

```bash
./maven skills list          # human-friendly list
./maven skills list --json   # stable JSON contract
./maven skills info writer
./maven skills info writer --json
./maven skills check         # validate the directory
./maven skills check --json
```

### JSON contract (stable)

All `--json` outputs share:

| Field | Type | Description |
|-------|------|-------------|
| `schemaVersion` | int | Currently `1`. |
| `command` | string | `skills.list` / `skills.info` / `skills.check`. |
| `ok` | bool | True when the command produced its intended output without error. |

Command-specific fields:

- **`skills list --json`**: `enabled`, `dir`, `loaded`, `skills[]` (each with `name`, `description`, `keywords[]`).
- **`skills info <name> --json`**: `name`, `description`, `dir`, `keywords[]`, `source`, `preview`, optional `handlerError`.
- **`skills check --json`**: `enabled`, `dir`, `skillFolders`, `loaded`, `missingSkillMD[]`, `result`, optional `note`.

## Errors

- **Missing frontmatter or invalid YAML** — the skill is skipped with a warning log (line, file path). The rest still load.
- **Empty `name`** — fatal during `LoadSkills` (the gateway logs but does not register that skill).
- **Duplicate `name` across folders** — fatal; the conflicting paths are reported.

## Reload

Skills are re-loaded on every `Apply`. With `gateway.hotReload = true` you can edit a skill file, save, and the gateway picks it up after the debounce. Without hot reload, restart `maven gateway`.

## Where activation happens

The skill matcher (keyword-based) lives in `ageneral-agents-go/pkg/runtime/skills`. Maven's loader (`internal/plugins/skill/file`) builds the `api.SkillRegistration` list and hands it to the SDK runtime via `apply.go`. Match logic is the SDK's responsibility.
