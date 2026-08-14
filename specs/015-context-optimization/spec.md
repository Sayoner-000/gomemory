# Feature Specification: Context Optimization & Budgeting

**Feature Branch**: `015-context-optimization`

**Created**: 2026-08-14

**Status**: Draft

**Input**: User description: "Feature Specification: Context Optimization & Budgeting — extend GoMemory with a Context Optimization Engine that retrieves, ranks, deduplicates, compresses, and budgets contextual information (persisted memories, Spec Kit artifacts, MCP tool descriptions) before it is handed to an AI agent/LLM, producing a deterministic, token-budgeted ContextPack via a Go API and a CLI, independent of any specific LLM provider or coding agent."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Build a task-scoped context package within a token budget (Priority: P1)

A person working with an AI coding agent is about to hand it a task. Instead of dumping the project's entire memory history at the agent, they ask GoMemory for "the context needed to do X" with an explicit token budget. GoMemory returns only the information relevant to that task, fitted inside the budget, instead of everything it has ever stored.

**Why this priority**: This is the core value proposition of the feature — everything else (deduplication, compression, Spec Kit ingestion, tool optimization) is a refinement on top of this basic retrieve-and-budget capability. Without it, there is no Context Optimization Engine, only a memory store.

**Independent Test**: Given a project with a mix of relevant and irrelevant stored memories and a task description with a token budget, request a context package and confirm it contains the relevant items, omits the irrelevant ones, and its total token count does not exceed the requested budget.

**Acceptance Scenarios**:

1. **Given** a project with stored memories on several unrelated topics, **When** a context package is requested for a specific task with a token budget, **Then** the returned package contains only memories relevant to that task and its total token count is within the requested budget.
2. **Given** a task whose truly essential ("critical") information alone exceeds the requested budget, **When** a context package is requested, **Then** the system reports an explicit budget-overflow error instead of silently omitting critical information.
3. **Given** a context package has been produced, **When** its contents are inspected, **Then** each included item states which original memory it came from, so the source can be traced back.

---

### User Story 2 - Remove duplicated information before it consumes budget (Priority: P2)

A project accumulates many memories over time that restate the same fact in slightly different words (e.g., two separate notes both saying the project uses a particular database). Before token budget is spent, the person wants near-identical information collapsed into a single, best representative entry rather than being included twice.

**Why this priority**: Duplicate information is one of the most direct and easy-to-verify sources of wasted budget identified in the problem statement. It builds directly on User Story 1's retrieval step and materially increases the amount of *useful, distinct* information that fits in a given budget.

**Independent Test**: Given two or more stored memories that state the same fact in different wording, request a context package covering that topic and confirm only one representative entry appears in the result, and that the discarded duplicates are reflected in the package's statistics rather than silently vanishing.

**Acceptance Scenarios**:

1. **Given** two memories with identical text, **When** a context package is built, **Then** only one copy appears in the result.
2. **Given** two memories that restate the same fact with different wording, **When** a context package is built, **Then** the system treats them as likely duplicates and retains the higher-quality one, and the discarded one is counted as a removed duplicate in the package's statistics.

---

### User Story 3 - See how much context was reduced and why (Priority: P3)

After requesting a context package, the person wants a clear before/after picture: how many tokens the raw candidate information would have cost, how many tokens the final package actually costs, how much was saved, and how many items were kept versus dropped and at which priority level. This lets them trust the system and tune budgets/settings with evidence instead of guesswork.

**Why this priority**: Observability is what makes the optimization trustworthy and tunable; it depends on Stories 1 and 2 already producing a package, but does not require compression or Spec Kit integration to deliver value on its own.

**Independent Test**: Build a context package for a task, then request its statistics through the CLI or API and confirm the reported raw token count, final token count, savings, and item counts (critical/relevant/optional/discarded) match the actual package produced.

**Acceptance Scenarios**:

1. **Given** a context package has been built, **When** its statistics are requested, **Then** the report shows raw tokens, final tokens, tokens saved, and the reduction percentage, and these numbers are internally consistent (raw − saved = final).
2. **Given** a context package has been built, **When** its statistics are requested, **Then** the report breaks down how many items were treated as critical, relevant, optional, and discarded.
3. **Given** arbitrary text (not tied to a stored memory), **When** it is submitted for compression alone, **Then** the response shows the token count before and after compression for that text.

---

### User Story 4 - Pull in only the relevant parts of the current Spec Kit feature (Priority: P4)

A person is running `/plan` or `/implement` for a specific Spec Kit feature. They want the context package to automatically include the relevant slices of that feature's constitution constraints, requirements, decisions, and task dependencies — and to leave out unrelated features' specs and historical discussions that live in the same project.

**Why this priority**: This extends Story 1's retrieval to a second, structured source of truth (Spec Kit artifacts) that this project already produces as part of its own workflow. It is valuable on its own once basic retrieval/budgeting exists, but the project can ship and prove value without it first.

**Independent Test**: In a project with multiple Spec Kit features, request a context package for a task belonging to one feature and confirm the package includes that feature's relevant constitution/requirement/decision/task content and excludes unrelated features' specs, with Spec Kit inclusion able to be turned off entirely for a request.

**Acceptance Scenarios**:

1. **Given** a project with two or more Spec Kit features, **When** a context package is requested for a task tied to one feature, **Then** the package includes that feature's relevant requirements/decisions/constraints and excludes the other features' specs.
2. **Given** a context package request that explicitly disables Spec Kit inclusion, **When** the package is built, **Then** no Spec Kit artifact content appears in the result.

---

### User Story 5 - Shrink verbose tool descriptions without changing how tools behave (Priority: P5)

An integrator has a set of MCP tool definitions with long, verbose descriptions. They want the descriptive text trimmed down to reduce the token overhead of exposing those tools, while every functional part of the tool definition (its name, parameters, required fields, types, enum values, and schema) stays byte-for-byte usable by the calling agent.

**Why this priority**: This is the most self-contained and lowest-blast-radius capability — it operates on tool metadata rather than the memory/Spec Kit retrieval pipeline — and is the least critical to the core "smallest useful context" goal, so it can safely ship last.

**Independent Test**: Given a tool definition with a verbose description and a defined parameter schema, run it through the optimizer and confirm the description is shorter while the name, parameters, required fields, types, enum values, and schema are unchanged.

**Acceptance Scenarios**:

1. **Given** a tool definition with a long, redundant description, **When** it is optimized, **Then** the resulting description is shorter while conveying the same core purpose.
2. **Given** a tool definition with a parameter schema, **When** it is optimized, **Then** the name, parameter names, parameter types, required fields, enum values, and schema are identical to the input.

---

### Edge Cases

- What happens when the requested budget is smaller than the smallest possible critical item? The system MUST fail with an explicit overflow error rather than returning a truncated critical item or an empty package presented as success.
- What happens when compression of a piece of content fails or errors out? The system MUST fall back to using that content's original, uncompressed form and continue building the package rather than aborting the whole request.
- What happens when no candidate information is relevant to the task at all? The system MUST return a valid, empty (or near-empty) package with statistics that make the absence of content explicit, not an error.
- What happens when Spec Kit inclusion is requested but no Spec Kit artifacts exist in the project? The system MUST proceed using the other enabled sources without treating the missing artifacts as an error.
- What happens when a project has never enabled or used Context Optimization? The system MUST behave exactly as it did before this feature existed — existing retrieval/save/search behavior is unaffected unless a context package is explicitly requested.
- What happens when two candidate items are judged "possibly duplicate" but a person would consider them meaningfully different on closer reading? The system MUST keep both when similarity is below the retained-duplicate threshold rather than aggressively merging borderline cases, since silently dropping real information is worse than an occasional near-duplicate surviving.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST accept a context request consisting, at minimum, of a task description and a maximum token budget, and MUST produce a single context package as the result.
- **FR-002**: The system MUST retrieve candidate information relevant to the task description from the project's stored memories.
- **FR-003**: The system MUST score and rank retrieved candidates by relevance to the task before deciding what to include.
- **FR-004**: The system MUST detect duplicated or near-duplicated information among candidates and retain only the highest-quality representative of each duplicate group, while recording how many items were removed as duplicates.
- **FR-005**: The system MUST classify every candidate item into exactly one of three priority levels — critical (must be preserved exactly), relevant (may be shortened), or optional (may be shortened or dropped) — before allocating budget.
- **FR-006**: The system MUST measure the token cost of candidate content, and MUST report, per package, the raw token count, the final token count, the tokens saved, and the reduction percentage.
- **FR-007**: The system MUST allocate the requested token budget to items in priority order (critical first, then relevant, then optional) and MUST NOT exceed the requested budget in the final package.
- **FR-008**: The system MUST NOT silently drop or shorten critical-priority content to make it fit the budget. If the full set of critical content cannot fit within the requested budget, the system MUST reject the request with an explicit, distinguishable overflow error rather than returning a partial or misleading package.
- **FR-009**: The system MUST be able to shorten relevant- and optional-priority content deterministically (without depending on an external AI/LLM call) as its baseline compression capability, and this deterministic shortening MUST NOT alter code blocks, commands, URLs, file paths, identifiers, error messages, numeric values, or version numbers contained in the content.
- **FR-010**: The system MUST leave a way to recover each item's original, uncompressed content — compressing content for a package MUST NOT overwrite or lose the source memory it came from.
- **FR-011**: If shortening a piece of content fails for any reason, the system MUST continue building the package using that content's original form rather than failing the whole request.
- **FR-012**: The system MUST expose the context-package capability through both a programmatic interface usable from other GoMemory-integrated code and a command-line interface usable directly by a person.
- **FR-013**: The command-line interface MUST support, at minimum: building a context package for a task, inspecting/re-displaying a previously built package, compressing a piece of arbitrary text on its own, and reporting reduction statistics.
- **FR-014**: The system MUST NOT assume or require any specific LLM provider, and MUST NOT assume or require any specific coding agent or agent CLI, in order to produce a context package.
- **FR-015**: The system MUST allow Spec Kit artifact ingestion (constitution constraints, feature requirements, decisions, and task dependencies) to be included in a context request, scoped to the task's feature rather than the whole project, and this inclusion MUST be able to be turned off per request.
- **FR-016**: The system MUST support restricting retrieval to a named scope (e.g., a specific project or a sub-area of a project) so unrelated projects' or areas' information does not contaminate a context package.
- **FR-017**: The system MUST be able to shorten a tool's description text without altering that tool's name, parameter names, parameter types, required fields, enum values, or schema.
- **FR-018**: Producing a context package MUST NOT change the behavior of GoMemory's existing memory save/search/retrieve capabilities for callers that do not request a context package — the feature is additive and must be explicitly invoked.
- **FR-019**: The system MUST make its retrieval, budgeting, and compression behavior configurable (at minimum: default token budget, minimum relevance threshold, maximum candidate items, compression aggressiveness, whether deduplication is active, and whether Spec Kit inclusion is active) without requiring a code change to adjust.

### Key Entities

- **Context Request**: The input to the engine — what task is being worked on, which project/scope it belongs to, the token budget, and which optional sources (memories, Spec Kit, tool descriptions) and compression behavior to apply.
- **Context Package**: The output of the engine for a given request — the ordered set of included items, the total and raw token counts, the achieved reduction, the configured budget, and where each item came from. This is what gets handed to the agent/LLM.
- **Context Item**: A single unit of information inside a package — its content (possibly shortened), which stored memory or artifact it originated from, its relevance/importance/confidence, its priority level (critical/relevant/optional), its token cost before and after shortening, and whether it was shortened.
- **Package Statistics**: The reduction report attached to a package — raw tokens, retrieved tokens, shortened tokens, final tokens, tokens saved, reduction percentage, and item counts per priority level plus items discarded.
- **Spec Kit Feature Context**: The structured slice of a Spec Kit feature (constitution constraints, requirements, decisions, task dependencies) that can feed into a context request, scoped to one feature.
- **Tool Description**: The name, parameter schema, and description text of an MCP-style tool, as input to and output from the description-shortening capability.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a project with a realistic mix of accumulated memories, a context package built for a specific task uses at least 30% fewer tokens than sending all retrieved candidate information unshortened, while still containing everything marked critical for that task.
- **SC-002**: When two or more stored memories restate the same fact, requesting a context package on that topic returns that fact once, not once per restatement — duplicate-driven token waste drops by at least 20% on projects with known repeated content.
- **SC-003**: Critical information is never observed missing from a produced package; when it cannot fit the requested budget, the request instead fails with a clear, distinguishable error 100% of the time.
- **SC-004**: A person can determine, without reading any source code, how much a given context package was reduced and how many items were kept versus dropped, using only the CLI or the package's reported statistics.
- **SC-005**: A project that never explicitly requests a context package shows zero difference in existing memory save/search/retrieve behavior before and after this feature is available.
- **SC-006**: Deterministic (non-AI) shortening of the same input content always produces the same output content, across repeated runs.
- **SC-007**: Building a context package from locally stored memories completes in under 100ms of added processing time beyond the existing retrieval calls it builds on, for typical project sizes.
- **SC-008**: A tool description run through the shortening capability keeps its name, parameters, required fields, types, enum values, and schema byte-for-byte identical to the input, verified across a representative set of tool definitions.

## Assumptions

- Candidate context sources for the first implementation are GoMemory's own stored memories, Spec Kit artifacts (constitution/spec/plan/tasks), and MCP-style tool descriptions. Reading raw project source code directly (beyond what a memory or Spec Kit artifact already references by file path) is out of scope for this feature; existing code-graph tooling in this project is a separate, already-existing capability and is not a new source this feature needs to ingest.
- "Relevance to the task" is computed using the text-matching/ranking capability GoMemory already has for searching stored memories (keyword/full-text ranking), combined with simple, locally computable signals such as how recently a memory was created and its stored type — no call to an external embedding or LLM service is required for the baseline (first-implementation) ranking.
- Deterministic, non-AI shortening (structural compression: collapsing repeated whitespace, redundant headers, duplicated paragraphs, etc.) is the only shortening strategy required for the first implementation. Shortening that requires calling an LLM is an optional, pluggable capability that projects can add later; its absence does not block this feature from being considered complete.
- Duplicate detection for the first implementation relies on exact and normalized (case/whitespace-insensitive) text matching. Similarity-based duplicate detection beyond exact/normalized matching is a nice-to-have refinement, not a blocking requirement, given no existing embedding infrastructure for stored memories today.
- A context package request is always explicit and opt-in (via the CLI or the programmatic API described in FR-012); nothing in this feature silently changes what an agent receives through GoMemory's existing, already-in-use retrieval calls.
- Configuration of default budgets and thresholds is per-project and has sensible built-in defaults; a person does not have to configure anything to get a usable, reduced context package on the first call.
- "MCP tool description optimization" (User Story 5) applies to tool metadata supplied to the optimizer as input; it does not require this feature to intercept or proxy live MCP protocol traffic.
