---
name: adversarial-consensus-review
description: >
  Perform an evidence-driven adversarial review using two independent reviewers,
  consensus-based defect confirmation, bounded correction rounds, and independent
  re-verification. Use when high-confidence validation is required for code,
  specifications, plans, architecture, configurations, migrations, contracts,
  or other concrete technical artifacts.
license: Apache-2.0
metadata:
  version: "1.0.0"
  category: review
  mode: adversarial-consensus
  portability: vendor-neutral
---

# Adversarial Consensus Review

Perform a bounded adversarial review of a concrete technical artifact using two
independent reviewers.

The goal is not to maximize the number of findings.

The goal is to identify defects supported by evidence, independently corroborate
them, correct only sufficiently confirmed problems, and verify those corrections
without expanding the original scope unnecessarily.

The canonical workflow is:

    TARGET
       │
       ▼
    FREEZE
       │
       ├───────────────┐
       ▼               ▼
    REVIEWER A      REVIEWER B
    read-only       read-only
       │               │
       └───────┬───────┘
               ▼
         CONSENSUS LEDGER
               │
       ┌───────┼───────────┐
       ▼       ▼           ▼
    CONFIRMED SUSPECT  CONTRADICTION
       │
       ▼
    FIX ACTOR
       │
       ▼
    FIX DELTA
       │
       ├───────────────┐
       ▼               ▼
    REVIEWER A      REVIEWER B
       │               │
       └───────┬───────┘
               ▼
          FINAL VERDICT
               │
      ┌────────┼─────────┐
      ▼        ▼         ▼
  APPROVED  ESCALATED INCOMPLETE


# 1. Activation

Use this skill when the user explicitly requests or clearly requires:

- adversarial review;
- independent review;
- dual review;
- consensus review;
- cross-checking;
- second opinion;
- deep validation;
- high-confidence review;
- independent verification;
- pre-merge validation;
- pre-release technical validation;
- review of a critical technical artifact.

A concrete review target MUST exist.

Possible targets include:

- source code;
- patches;
- diffs;
- commits;
- pull request changes;
- specifications;
- implementation plans;
- task plans;
- architecture documents;
- API contracts;
- database migrations;
- infrastructure configuration;
- security-sensitive configuration;
- generated artifacts;
- technical design documents.

Do not activate this skill merely because ordinary review is possible.


# 2. Objectives

The protocol MUST:

1. ensure that two reviewers inspect the same target;
2. preserve independence between reviewers;
3. require evidence for actionable findings;
4. distinguish corroborated defects from unconfirmed observations;
5. prevent reviewers from modifying the target;
6. isolate corrective work from review work;
7. restrict corrections to explicitly confirmed findings;
8. perform independent re-verification after corrections;
9. bound correction loops;
10. preserve a clear terminal state;
11. remain independent of any specific model provider, agent framework, editor,
    CLI, IDE, memory system, or orchestration runtime.


# 3. Non-Goals

This skill is not:

- a generic linter;
- an automatic formatter;
- a style checker;
- unrestricted autonomous refactoring;
- a replacement for automated tests;
- a replacement for CI/CD;
- a substitute for human approval in unresolved high-risk decisions;
- authorization to commit, push, merge, deploy, publish, or release;
- a mechanism for storing private chain-of-thought;
- a mechanism for forcing agents to invent findings;
- a dependency on any specific AI vendor or runtime.


# 4. Core Invariants

The following invariants MUST hold for every review.

## INV-001 — Same Target

Both reviewers MUST inspect the same frozen target.

## INV-002 — Independent Judgment

Neither reviewer may receive the other reviewer's findings before completing its
own initial judgment.

## INV-003 — Read-Only Review

Reviewers MUST NOT modify the target.

## INV-004 — Evidence Required

A finding cannot become actionable solely because it sounds plausible.

## INV-005 — Independent Corroboration

A confirmed finding requires independent corroboration.

## INV-006 — Bounded Correction

Only authorized confirmed findings may trigger corrective work.

## INV-007 — Fix Traceability

Every correction MUST reference the finding or findings it addresses.

## INV-008 — Re-Verification

Corrections MUST be independently verified.

## INV-009 — Round Budget

Correction and verification loops MUST be bounded.

## INV-010 — Fail Closed

Incomplete review execution MUST NOT produce an APPROVED verdict.

## INV-011 — Delivery Separation

Review approval MUST NOT imply authorization to commit, merge, deploy, publish,
or release.

## INV-012 — No Chain-of-Thought Persistence

Private reasoning or chain-of-thought MUST NOT be requested, required, persisted,
or used as a review artifact.


# 5. Terminology

## Target

The exact artifact being reviewed.

## Frozen Target

An immutable or sufficiently stable representation of the target used by all
reviewers in a round.

## Reviewer

An independent read-only agent responsible for defect discovery.

## Finding

A structured claim describing a potential defect.

## Evidence

Concrete information supporting a finding.

## Consensus Ledger

The normalized result produced by comparing reviewer findings.

## Confirmed Finding

A defect independently identified by both reviewers.

## Suspect Finding

A defect identified by only one reviewer.

## Contradiction

A case where reviewers produce materially incompatible conclusions about the same
behavior.

## Fix Actor

An agent permitted to modify the target to resolve explicitly authorized
confirmed findings.

## Fix Delta

The exact changes produced by corrective work.

## Re-Judgment

A scoped independent review of whether a confirmed defect was resolved and
whether its correction introduced a directly related regression.


# 6. Phase 0 — Resolve Applicable Context

Before reviewing the target, resolve the technical context necessary to judge it
correctly.

Relevant context may include:

- repository instructions;
- architecture decisions;
- specifications;
- project conventions;
- acceptance criteria;
- coding standards;
- API contracts;
- security constraints;
- design requirements;
- applicable skills;
- task definitions.

Both reviewers MUST receive materially equivalent project constraints.

Do not provide one reviewer with privileged information unavailable to the other
unless the difference is intrinsic to the target.


# 7. Phase 1 — Freeze the Target

Before launching reviewers, establish a stable target identity.

Use the strongest available identifier.

Preferred order:

1. immutable artifact digest;
2. commit or revision identifier;
3. tree hash;
4. exact diff;
5. exact file set and content digest;
6. stable document revision;
7. explicitly captured scope.

Record at minimum:

    target:
      type:
      identity:
      revision:
      digest:
      scope:

If no immutable identifier exists, record enough information to guarantee that
both reviewers inspect the same content.

If the target changes during the review, invalidate the round and restart from
the new frozen target.


# 8. Phase 2 — Independent Review

Create two logically independent reviewers:

    reviewer-a
    reviewer-b

Parallel execution is preferred when supported, but it is not required.

If only sequential execution is possible:

1. execute Reviewer A;
2. preserve its result without exposing it to Reviewer B;
3. execute Reviewer B from the original frozen target;
4. compare results only after both reviews finish.

Reviewer B MUST NOT be asked to:

- validate Reviewer A;
- confirm Reviewer A;
- critique Reviewer A;
- refute Reviewer A;
- rank Reviewer A's findings.

That would be sequential validation rather than independent review.


# 9. Reviewer Diversity

When the runtime supports model selection, prefer reviewer diversity.

Preferred order:

1. different model families;
2. different models;
3. different providers;
4. different reasoning configurations;
5. isolated executions of the same model.

Model diversity improves independence but is not mandatory.

Do not claim strong model independence when both reviewers share effectively the
same model and context.

When available, record:

    reviewer_a:
      provider:
      model:
      configuration:

    reviewer_b:
      provider:
      model:
      configuration:

    independence:
      level: full | partial | degraded
      reason:


# 10. Reviewer Permissions

Reviewers MUST operate in read-only mode.

They MAY:

- read files;
- inspect diffs;
- inspect history;
- search code;
- trace control flow;
- inspect specifications;
- inspect contracts;
- execute non-mutating diagnostic commands;
- run tests that do not modify the target;
- gather runtime evidence;
- analyze dependency relationships.

They MUST NOT:

- edit files;
- apply patches;
- rewrite documents;
- refactor code;
- commit;
- push;
- merge;
- create pull requests;
- deploy;
- publish;
- modify persistent memory;
- delegate corrective work;
- change the review target.


# 11. Review Criteria

Unless the user specifies narrower criteria, reviewers SHOULD inspect:

1. functional correctness;
2. behavioral regressions;
3. edge cases;
4. error handling;
5. state consistency;
6. concurrency;
7. race conditions;
8. resource lifecycle;
9. security boundaries;
10. authorization and permissions;
11. data integrity;
12. API compatibility;
13. contract compatibility;
14. backwards compatibility;
15. performance pathologies;
16. failure recovery;
17. implementation/specification consistency;
18. architecture violations;
19. unsafe assumptions;
20. repository-specific invariants.

Do not manufacture findings merely to cover every category.


# 12. Evidence-First Review

Every actionable finding MUST include concrete evidence.

Acceptable evidence includes:

- exact path and location;
- failing test;
- reproducible scenario;
- deterministic execution path;
- violated invariant;
- contract contradiction;
- invalid state transition;
- runtime trace;
- security boundary violation;
- incorrect API behavior;
- data corruption path;
- concurrency schedule;
- static analysis result.

Avoid treating the following as defects without stronger evidence:

- stylistic preference;
- personal design preference;
- vague suspicion;
- hypothetical scenarios with no reachable execution path;
- speculative performance concerns;
- optional refactors;
- alternative implementations;
- purely aesthetic improvements.


# 13. Severity Model

Use only the following severities.

## CRITICAL

A defect that can plausibly cause:

- security compromise;
- privilege escalation;
- unrecoverable data loss;
- large-scale corruption;
- catastrophic service failure;
- severe violation of a trust boundary.

## HIGH

A significant defect causing:

- reproducible functional failure;
- major behavioral regression;
- incorrect persistent state;
- broken external contract;
- serious concurrency defect;
- significant production failure;
- unsafe error handling.

## MEDIUM

A real defect with bounded impact.

Examples:

- incorrect edge-case behavior;
- localized inconsistency;
- incomplete validation;
- recoverable failure;
- bounded performance issue.

## LOW

A minor correctness or robustness defect with limited operational impact.

## INFO

A non-blocking observation.

Examples:

- readability;
- style;
- optional refactor;
- documentation suggestion;
- alternative design;
- unverified concern.


# 14. Finding Format

Each reviewer MUST return structured findings.

Preferred representation:

    {
      "findings": [
        {
          "id": "A-001",
          "location": "path/file.ext:120-137",
          "severity": "HIGH",
          "category": "concurrency",
          "claim": "Concurrent updates can overwrite a newer state.",
          "evidence_class": "deterministic",
          "evidence": [
            "The state is read before entering the protected update boundary.",
            "Two concurrent requests can observe the same previous value."
          ],
          "confidence": "high"
        }
      ],
      "inspected": [
        "path/file.ext",
        "path/file_test.ext"
      ]
    }

Reviewer identifiers SHOULD use:

    A-001
    A-002

and:

    B-001
    B-002

The exact serialization format MAY differ when the runtime requires another
structured format.


# 15. Evidence Classes

Use one of:

- deterministic;
- reproduced;
- contract;
- test-failure;
- static-analysis;
- runtime-observation;
- probabilistic.

Prefer:

    deterministic
    reproduced
    contract
    test-failure

over weaker speculative evidence.

Probabilistic findings require stronger corroboration before corrective action.


# 16. Reviewer Prompt Contract

When spawning or instructing a reviewer, preserve the following behavioral
contract:

    You are an independent reviewer.

    Review the frozen target from first principles.

    Do not modify the target.

    Do not ask for another reviewer's findings.

    Do not assume another reviewer agrees with you.

    Do not manufacture findings.

    Every defect must describe concrete incorrect behavior and include evidence.

    Return structured findings only.

    If no meaningful defect exists, return an empty findings collection.

Runtime-specific syntax MAY differ, but these semantics MUST remain intact.


# 17. Phase 3 — Consensus

Only the orchestrator or consensus component may compare the reviewer outputs.

It MUST:

1. validate reviewer result structure;
2. reject malformed findings when necessary;
3. normalize equivalent terminology;
4. match findings by underlying defect;
5. preserve disagreement;
6. preserve unsupported single-reviewer observations;
7. generate the consensus ledger.

The consensus process MUST NOT invent new defects.


# 18. Finding Equivalence

Two findings SHOULD be considered equivalent when they describe substantially
the same:

- failing behavior;
- causal mechanism;
- affected execution path;
- affected invariant;
- affected component.

Textual similarity alone is insufficient.

Example:

Reviewer A:

    Concurrent writes can overwrite state.

Reviewer B:

    Two requests can persist updates derived from the same stale state.

These findings may represent the same underlying defect even though their wording
differs.


# 19. Consensus Classes

Each normalized finding MUST be classified as one of:

## CONFIRMED

Both reviewers independently identified the same underlying defect.

## SUSPECT

Only one reviewer identified the defect.

## CONTRADICTION

The reviewers reached incompatible conclusions about the same behavior or
invariant.

## INFO

Non-blocking observation.


# 20. Consensus Ledger

The orchestrator SHOULD produce a ledger equivalent to:

    target:
      identity:
      revision:
      digest:

    reviewers:
      reviewer_a:
        model:
      reviewer_b:
        model:

    consensus:

      confirmed:
        - id: C-001
          severity: HIGH
          sources:
            - A-001
            - B-003
          claim:
          evidence:
          status: open

      suspect:
        - id: S-001
          source: A-004
          severity: MEDIUM
          claim:

      contradictions:
        - id: X-001
          subject:
          reviewer_a:
          reviewer_b:

      info: []


# 21. Correction Policy

By default, automatic or pre-authorized corrective work is permitted only for:

    CONFIRMED + CRITICAL
    CONFIRMED + HIGH

The following MUST NOT trigger automatic correction by default:

    SUSPECT
    CONTRADICTION
    MEDIUM
    LOW
    INFO

The user or calling system MAY define a stricter or broader policy.

Contradictory severe findings SHOULD be escalated rather than guessed.


# 22. Human Authorization

If the user requested review only, do not modify the target.

If the user explicitly requested:

- review and fix;
- review until approved;
- fix confirmed defects;
- validate and correct;

then corrective work may proceed according to the configured policy.

Review authorization and write authorization are separate concepts.


# 23. Phase 4 — Fix Actor

Corrective work SHOULD be performed by an actor separate from the initial
reviewers when the runtime supports delegation.

The Fix Actor receives only:

- frozen target identity;
- authorized confirmed findings;
- evidence for those findings;
- applicable project rules;
- allowed correction scope.

The Fix Actor SHOULD NOT receive unrelated suspect findings unless explicitly
authorized.


# 24. Minimal Correction Principle

For every authorized finding:

1. identify the smallest sufficient correction;
2. modify only necessary scope;
3. preserve unrelated behavior;
4. add or update focused verification when appropriate;
5. execute relevant validation;
6. record changed artifacts;
7. record which finding the change addresses.

Do not perform opportunistic cleanup.


# 25. Fix Actor Restrictions

The Fix Actor MUST NOT:

- fix SUSPECT findings without authorization;
- resolve INFO observations opportunistically;
- perform unrelated refactors;
- redesign unrelated architecture;
- expand public APIs unnecessarily;
- alter unrelated files for cleanup;
- commit;
- push;
- merge;
- deploy;
- release;

unless separately authorized by the calling workflow.


# 26. Fix Delta

After corrective work, freeze the resulting delta.

Record:

    fix_delta:
      base_target:
      fixed_target:
      findings_addressed:
      modified_paths:
      diff_identity:
      verification:

The fixed target becomes a new immutable review revision.


# 27. Phase 5 — Re-Judgment

After correction, two reviewers MUST independently verify the fix.

Re-judgment is intentionally narrower than the initial review.

Inspect:

1. whether the original confirmed defect was resolved;
2. whether the relevant invariant now holds;
3. whether the correction directly introduced a regression.

Do not silently turn re-judgment into an unrestricted new code review.


# 28. Re-Judgment Finding States

For every confirmed finding, reviewers MUST classify the result as:

    RESOLVED
    UNRESOLVED
    REGRESSED

A new finding may be introduced during re-judgment only when it is causally
related to the correction.

Unrelated observations belong to another review cycle.


# 29. Re-Judgment Prompt Contract

Use semantics equivalent to:

    You are independently verifying a correction.

    Inspect the original confirmed defect and the fix delta.

    Determine whether the defect is RESOLVED, UNRESOLVED, or REGRESSED.

    Check for regressions caused directly by the correction.

    Do not perform an unrelated full review.

    Do not modify the target.

    Return structured evidence.


# 30. Round Budget

The protocol MUST be bounded.

Default maximum:

    initial_review: 1
    fix_rounds: 2
    rejudgment_rounds: 2

Canonical maximum flow:

    INITIAL REVIEW
         │
         ▼
      FIX #1
         │
         ▼
    RE-JUDGMENT
         │
         ▼
      FIX #2
         │
         ▼
    RE-JUDGMENT
         │
         ▼
      ESCALATE

Never silently extend the configured budget.


# 31. Failure Handling

## Reviewer Unavailable

If one required reviewer cannot execute:

    VERDICT: INCOMPLETE

Do not substitute one review for two.

## Target Unavailable

If a reviewer cannot inspect the required target:

    VERDICT: INCOMPLETE

## Target Changed

Invalidate the current round and restart against the new frozen target.

## Duplicate Reviewer Execution

If two outputs originate from the same effective execution or one output was
copied from another, treat them as one review.

## Malformed Reviewer Result

Attempt only bounded normalization.

If the result cannot be interpreted reliably:

    VERDICT: INCOMPLETE

## Severe Contradiction

If reviewers disagree materially on a CRITICAL or HIGH behavior:

    VERDICT: ESCALATED

unless an explicitly authorized arbitration mechanism exists.


# 32. Independence Levels

Report the effective independence when possible.

## FULL

Examples:

- separate agents;
- isolated contexts;
- different models;
- equivalent access to the same frozen target.

## PARTIAL

Examples:

- isolated agents using the same model;
- sequential independent execution with clean context boundaries.

## DEGRADED

Examples:

- one runtime performing two simulated passes;
- context isolation cannot be guaranteed;
- same agent must emulate both reviewer roles.

When independence is degraded, state it explicitly.

Never present degraded review as fully independent multi-agent review.


# 33. Runtime Adaptation

This skill MUST remain runtime-agnostic.

A runtime may implement reviewer isolation using:

- subagents;
- worker agents;
- child sessions;
- isolated model invocations;
- agent pools;
- remote agents;
- separate CLI processes;
- tool-driven executions;
- workflow nodes.

The implementation mechanism is irrelevant as long as the protocol invariants
hold.


# 34. No-Subagent Fallback

If the runtime lacks subagents but can perform isolated passes:

1. preserve the frozen target;
2. execute Review Pass A;
3. remove its findings from active context;
4. execute Review Pass B independently;
5. compare only after both passes complete.

Report:

    independence: degraded

or:

    independence: partial

depending on the actual isolation guarantees.

If genuine isolation cannot be approximated responsibly, prefer:

    VERDICT: INCOMPLETE

over pretending the protocol was fully executed.


# 35. Final Verification

Final verification is separate from initial defect discovery.

Before APPROVED, confirm:

- both required reviewers executed;
- both inspected the same target revision;
- confirmed severe findings are resolved;
- no unresolved severe contradiction remains;
- required focused tests passed;
- broader relevant validation passed when feasible;
- the correction remained within authorized scope;
- no correction-caused severe regression remains;
- the round budget was respected.


# 36. Terminal States

Only the following terminal states are valid:

    APPROVED
    ESCALATED
    INCOMPLETE

## APPROVED

Use when:

- the required independent review completed;
- no unresolved confirmed CRITICAL or HIGH finding remains;
- no unresolved severe contradiction remains;
- required correction verification completed.

## ESCALATED

Use when:

- a severe defect remains after the allowed correction rounds;
- reviewers materially disagree on a severe issue;
- correction requires judgment beyond the allowed scope;
- human intervention is required.

## INCOMPLETE

Use when:

- a required reviewer could not execute;
- target identity could not be preserved;
- reviewer evidence is unusable;
- protocol invariants could not be satisfied.


# 37. Final Output

Preferred result:

    ADVERSARIAL CONSENSUS REVIEW

    Target:
    Revision:
    Independence:
    Review rounds:
    Fix rounds:

    Confirmed:
    Suspect:
    Contradictions:
    Info:

    Corrections:
    Re-judgment:
    Verification:

    VERDICT: APPROVED

or:

    VERDICT: ESCALATED

or:

    VERDICT: INCOMPLETE

Return exactly one terminal verdict.


# 38. Memory and Persistence

This skill does not require a memory system.

If the runtime provides persistent memory, it MAY preserve reusable review
knowledge.

Appropriate knowledge includes:

- confirmed defect pattern;
- affected component;
- root cause summary;
- resolution summary;
- prevention rule;
- validation performed;
- final verdict.

Do NOT persist:

- private chain-of-thought;
- hidden reasoning;
- speculative reviewer deliberation;
- duplicated raw transcripts;
- irrelevant temporary observations.

Prefer distilled knowledge over conversation logs.


# 39. Reusable Review Learning

A resolved confirmed finding MAY be transformed conceptually into:

    review_learning:
      category:
      component:
      problem:
      root_cause:
      resolution:
      prevention:
      verification:
      confidence:

Promotion SHOULD generally require:

    CONFIRMED
    AND
    RESOLVED
    AND
    REUSABLE

Memory behavior is optional and MUST NOT affect the correctness of the review
protocol.


# 40. Security

Reviewers and corrective actors MUST obey the security boundaries of the calling
environment.

Do not request elevated privileges merely to complete the protocol.

Never expose or persist:

- credentials;
- authentication tokens;
- private keys;
- secrets;
- unnecessary sensitive information;
- private chain-of-thought.

If security-sensitive information is encountered, reference only the minimum
evidence required to describe the defect safely.


# 41. Delivery Boundary

An APPROVED verdict means only:

    the reviewed target satisfied this review protocol.

It does NOT mean:

    commit authorized
    push authorized
    PR authorized
    merge authorized
    deployment authorized
    production release authorized
    publication authorized

Those actions require separate instructions or workflow policy.


# 42. Recommended Orchestration

When full agent capabilities exist, prefer:

    ORCHESTRATOR
        │
        ├── REVIEWER A
        │      read-only
        │
        ├── REVIEWER B
        │      read-only
        │
        └── FIX ACTOR
               write only when authorized

The orchestrator owns:

- target freezing;
- reviewer isolation;
- result collection;
- consensus;
- correction authorization;
- round accounting;
- terminal verdict.

Reviewers own:

- independent defect discovery;
- evidence production.

The Fix Actor owns:

- bounded correction.

No single role should silently assume all three responsibilities when role
separation is available.


# 43. Recommended Invocation Examples

Code:

    Perform an adversarial consensus review of the current diff.

Specification:

    Use adversarial-consensus-review on this specification before implementation.

Architecture:

    Independently review this architecture with two reviewers and report only
    corroborated high-confidence defects.

Implementation plus correction:

    Implement the requested change, then run adversarial-consensus-review.
    Correct confirmed HIGH or CRITICAL defects and re-verify them before
    completion.

Review only:

    Review the current implementation using adversarial-consensus-review.
    Do not modify anything.

Pre-delivery:

    Run adversarial-consensus-review before considering this work ready for
    delivery. Do not commit or push.


# 44. Reviewer Template

Use an equivalent template when the runtime requires explicit reviewer
instructions:

    ROLE

    You are independent Reviewer {A|B}.

    TARGET

    {frozen target identity}

    SCOPE

    {exact review scope}

    PROJECT CONTEXT

    {applicable rules and contracts}

    CONTRACT

    - Review independently.
    - Do not modify the target.
    - Do not request another reviewer's findings.
    - Do not infer another reviewer's conclusions.
    - Do not manufacture defects.
    - Every actionable finding requires concrete evidence.
    - Return structured findings.
    - An empty findings set is valid.

    OUTPUT

    {
      "findings": [
        {
          "id": "{A|B}-001",
          "location": "",
          "severity": "CRITICAL|HIGH|MEDIUM|LOW|INFO",
          "category": "",
          "claim": "",
          "evidence_class": "",
          "evidence": [],
          "confidence": "high|medium|low"
        }
      ],
      "inspected": []
    }


# 45. Fix Actor Template

Use equivalent semantics:

    ROLE

    You are the bounded correction actor.

    TARGET

    {target identity}

    AUTHORIZED FINDINGS

    {confirmed findings authorized for correction}

    CONTRACT

    - Correct only authorized confirmed findings.
    - Use the smallest sufficient change.
    - Preserve unrelated behavior.
    - Do not perform opportunistic cleanup.
    - Add focused verification when appropriate.
    - Record modified artifacts and validation.
    - Do not commit, push, merge, deploy, or release unless separately authorized.

    OUTPUT

    {
      "work_units": [
        {
          "finding_id": "C-001",
          "modified_paths": [],
          "change_summary": "",
          "verification": []
        }
      ]
    }


# 46. Re-Judgment Template

Use equivalent semantics:

    ROLE

    You are independent Reviewer {A|B} performing scoped re-verification.

    ORIGINAL FINDING

    {confirmed finding}

    FIX DELTA

    {exact corrective delta}

    CONTRACT

    Determine whether the original defect is:

    - RESOLVED
    - UNRESOLVED
    - REGRESSED

    Check only the original defect, relevant invariants, and regressions directly
    introduced by the fix.

    Do not perform an unrelated full review.

    Do not modify the target.

    OUTPUT

    {
      "finding_id": "C-001",
      "status": "RESOLVED|UNRESOLVED|REGRESSED",
      "evidence": []
    }


# 47. Design Principle

The fundamental rule of this skill is:

> Independent evidence precedes consensus, consensus precedes correction, and
> correction precedes re-verification.

A reviewer is not valuable because it produces more findings.

A reviewer is valuable when it identifies a concrete defect that can be
explained, evidenced, independently corroborated, corrected with bounded scope,
and verified afterward.

When the runtime offers stronger isolation, multiple models, subagents, memory,
or orchestration capabilities, use them to strengthen the protocol.

Never make the protocol dependent on them.
