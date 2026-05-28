# Memory

Maven's memory has two layers — **long-term** (always in context) and **episodic** (retrieved on demand) — and one background pass that promotes facts from one to the other.

## Layers

| Layer | Backed by | When | Tool to write | Tool to read |
|-------|-----------|------|---------------|--------------|
| Long-term | `memory/MEMORY.md` | Curated; rarely changed. | Direct file edit by the agent (via SDK file tools). | Always injected after the system prompt. |
| Episodic | `memory/YYYY-MM-DD.md` | One file per UTC date. | `remember(content)` | `memory_search(query)`, `memory_get(date)` |

The split keeps the system prompt small while making yesterday's notes addressable.

## Long-term: `MEMORY.md`

Read at every gateway `Apply` (start, reload). Whatever is in the file is appended to the system prompt under the heading `# Long-term Memory`. The agent edits the file directly using SDK file tools (no special protocol).

The plugin in `internal/plugins/memory/file` declares itself primary, so it is the only memory writer in the registry. The `kernel/memory.Registry` enforces "exactly one primary" at construction time.

A truncation cap (`memoryMdMaxChars = 12288`) protects the system prompt from runaway memory files; over-cap content is suffixed with `…`.

## Episodic: daily journals

`memory/2026-05-27.md` is plain markdown — one entry per line is conventional but not required. The `remember` tool appends; nothing else is touched.

Path discipline:

- Filenames must be `YYYY-MM-DD.md`. Anything else is ignored.
- `MEMORY.md` is **never** treated as a journal.
- Sort order is reverse-chronological (newest first) for search and listing.

## Tools

### `remember(content)`

```json
{ "name": "remember", "input": { "content": "User prefers Postgres on Hetzner, not RDS." } }
```

Appends a line to today's journal (UTC). Creates the journal file (and `memory/` directory) on first write.

### `memory_search(query, limit?)`

```json
{ "name": "memory_search", "input": { "query": "postgres", "limit": 5 } }
```

Case-insensitive substring match across journal files. Returns up to `limit` (default 5) matching entries, newest first, with snippets truncated to 500 characters.

### `memory_get(date)`

```json
{ "name": "memory_get", "input": { "date": "today" } }
```

Accepts `today`, `yesterday`, or `YYYY-MM-DD`. Returns the file content with a heading or "No journal entry for {date}." when absent.

## Background consolidation

When enabled, `memconsolidate` reviews recent journals and asks the model to promote worth-keeping facts to `MEMORY.md`.

```json
{
  "memConsolidate": {
    "enabled": true,
    "intervalHours": 24
  }
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master toggle. |
| `intervalHours` | `24` | Wall-clock interval between passes. |

### What it does

On each tick:

1. Lists journal files from the last 7 days (excluding `MEMORY.md`).
2. Concatenates them (truncated at 4000 chars per file) into a single review prompt.
3. Runs one isolated agent turn (`isolated:mem-consolidate:{nanos}` session) with the long-term memory already in the system prompt.
4. The model decides whether to rewrite `memory/MEMORY.md` via the SDK file tools. The instruction is explicit: *"only include what clearly should persist indefinitely. If nothing new warrants promotion, leave the file unchanged."*

The consolidation pass uses the same `Apply`-built runtime and obeys the same admission lane (try-once weight-1 semaphore) as heartbeat.

### Skipped ticks

- No journal files in the last 7 days → no-op.
- Previous tick still running → skipped with a debug log.

## File layout

```text
workspace/memory/
  MEMORY.md            # long-term, curated
  2026-05-25.md        # journal
  2026-05-26.md
  2026-05-27.md
```

## Privacy

Memory files are plain markdown. No encryption, no secrets handling beyond the host filesystem. If you put credentials in a journal, the next consolidation pass might promote them to `MEMORY.md`. Don't.
