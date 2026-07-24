# Memory Protocol — Persistent Memory for Claude Code

You have access to **gomemory**, a persistent memory system for this project.
This protocol is MANDATORY and ALWAYS ACTIVE — not something you activate on demand.

## PROACTIVE SAVE (mandatory — do NOT wait for user to ask)

Call `save_memory` IMMEDIATELY after each of these:

| Trigger | Content to capture |
|---------|-------------------|
| Architecture/design decision | What was decided, why, tradeoffs |
| Bug fix completed | Root cause, what changed, files affected |
| Team convention established | The convention, why it was chosen |
| Tool/library choice | What was chosen, alternatives rejected, tradeoffs |
| Non-obvious codebase discovery | What was found, why it matters |
| Pattern established | The pattern, where it applies |
| User preference learned | The preference, context (type=preference — interactive/session memory: how the user wants to be worked with. Save it here, not in an external store). Fixed rule, not an incident log: reuse topic_key/title on repeat corrections to UPDATE, never quote the wrong-behavior examples in the content. |

Self-check after EVERY task: "Did I make a decision, fix a bug, discover
something non-obvious, or establish a convention? If yes → call save_memory NOW."

Format: title "Verb + what" (e.g. "Fixed N+1 query in user list")

## WHEN TO SEARCH

Call `search_memories` REACTIVELY when:
- User says "remember", "recall", "what did we do", "recordar"
- User references past work in any language

Call `search_memories` PROACTIVELY when:
- Starting work on something that might overlap past sessions
- Task mentions a topic you have no context on
- Before making a decision that might have been decided before

## PROGRESSIVE DISCLOSURE (TOKEN-EFFICIENT RETRIEVAL)

```text
1. search_memories(query) → compact results (~100 tokens each)
   Returns: ID, title, type, created_at

2. get_memory(id) → full untruncated content
   Only when you need detail

3. Never dump all memory — search first, drill only if needed
```

## AUTOMATIC CHECKPOINTS

Routine turn activity (which files were edited, which commands ran) is
captured automatically as a `checkpoint` memory after each turn — you don't
need to call `save_memory` for that. Keep using `save_memory` for things that
require synthesis: decisions, root causes, conventions, discoveries.

## IMPARTIAL JUDGE (conflicting memories)

If the context shows a `## Conflictos sin resolver` section, or you notice two
memories that contradict each other while searching, act as an impartial
judge:

1. Do NOT assume the more recent memory is correct.
2. Re-read the actual current code/source to verify which memory reflects
   reality.
3. Record the verdict: `judge_memories(id_a, id_b, verdict, confidence, reasoning)`
   — `reasoning` is required and must state what you verified.

## PRIVACY

Wrap secrets, tokens, or credentials in `<private>...</private>` before saving
— content inside those tags is stripped and never persisted.

## FORGETTING

If a memory is wrong, obsolete, or was saved by mistake, remove it with
`forget_memory(id)` instead of leaving stale/incorrect information around.

## SESSION CLOSE PROTOCOL (mandatory)

Before ending a session or saying "done", call `end_session()` with:

```
## Goal
[What we were working on this session]

## Discoveries
- [Technical findings, gotchas, non-obvious learnings]

## Accomplished
- [Completed items with key details]

## Next Steps
- [What remains to be done — for the next session]

## Relevant Files
- path/to/file — [what it does or what changed]
```

This is NOT optional. If you skip this, the next session starts blind.

## AFTER COMPACTION

If you see a compaction message or "FIRST ACTION REQUIRED":

1. IMMEDIATELY call `end_session(summary)` with the compacted summary
   content — this persists what was done before compaction
2. Call `get_context()` to recover previous session state
3. Only THEN continue working

Do not skip step 1. Without it, everything done before compaction
is lost from memory.
