# Contributing to the docs

User-facing docs live under `docs/` and publish to [https://ageneralai.github.io/maven/](https://ageneralai.github.io/maven/) via GitHub Pages (`gh-pages` branch). Go code changes use the normal PR flow; doc-only changes follow the steps below.

## Prerequisites

- Python 3.9+ with `pip`
- Git clone of [ageneralai/maven](https://github.com/ageneralai/maven) with push access if you will deploy
- Repo root is the MkDocs project (contains `mkdocs.yml` and `docs/`)

## One-time setup

From the repository root:

```bash
pip install -r requirements-docs.txt
```

Or install the same packages directly:

```bash
pip install mkdocs mkdocs-material
```

If `docs/` already exists, do **not** run `mkdocs new .` on top of this repo — it overwrites `mkdocs.yml`. The project is already initialized.

## Preview locally

```bash
mkdocs serve
```

Open [http://127.0.0.1:8000/](http://127.0.0.1:8000/). MkDocs reloads when you edit files under `docs/` or change `mkdocs.yml`.

Build static HTML without serving:

```bash
mkdocs build
```

Output goes to `site/` (gitignored).

## Add or edit a page

1. Add or edit a Markdown file under `docs/`, e.g. `docs/my-topic.md`.
2. Register it in `mkdocs.yml` under `nav:` so it appears in the sidebar. Unlisted files still build but MkDocs warns they are missing from nav.
3. Run `mkdocs serve` and fix any broken links or nav warnings before opening a PR.

Example nav entry:

```yaml
nav:
  - Home: index.md
  - My topic: my-topic.md
```

## Publish to GitHub Pages

Requires permission to push to `ageneralai/maven` and [GitHub Pages](https://docs.github.com/en/pages) enabled for the repo (source: `gh-pages` branch).

From the repository root, on `main` with doc changes merged or committed:

```bash
mkdocs gh-deploy --force
```

This builds `site/`, commits the result to the `gh-pages` branch, and pushes. The site updates at [https://ageneralai.github.io/maven/](https://ageneralai.github.io/maven/) after GitHub finishes processing (usually within a minute).

`--force` on `gh-deploy` overwrites the remote `gh-pages` branch when needed; use it when redeploying after nav or theme changes.

## What not to commit

- `site/` — build output (listed in `.gitignore`)
- Secrets or real tokens in doc examples

## Go code vs docs

| Change | Workflow |
|--------|----------|
| Markdown under `docs/` | Edit, update `nav` in `mkdocs.yml`, `mkdocs serve`, PR, then `mkdocs gh-deploy` (or ask a maintainer to deploy) |
| Go / config / channels | `make test`, normal PR; update docs in the same PR when behavior changes |

Questions about doc structure: open an issue or PR on [ageneralai/maven](https://github.com/ageneralai/maven).
