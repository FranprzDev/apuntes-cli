# apuntes-cli review rules

## Product

- Keep study content grounded in the source files provided by the user.
- Do not invent facts, citations, or academic references.
- Preserve the separation between subjects, sources, FAQs, and generated outputs.

## Workflow: stacked PRs

- Split large changes into a chain of small PRs, each branched off the previous one.
- Branch naming: `stack/<n>-<slug>`, e.g. `stack/1-refactor-cmd`, `stack/2-add-tests`.
- Base branch for each PR is the previous branch in the stack (first PR targets `main`).
- Open PRs with `gh pr create --base <previous-branch>`.
- When a lower PR changes, restack the ones above it:
  `git rebase --onto stack/<n>-updated stack/<n>-old stack/<n+1>-slug`
- Keep each commit atomic and independently reviewable.
- Only merge the bottom of the stack; after merging, retarget and restack the next PR onto `main` before merging it.

## Autonomous workflow

When given an open-ended or ambiguous prompt:

1. Explore the repo and decompose the goal into a concrete task list (use the todo list tool).
2. Work tasks one at a time. Each logical unit of work becomes one PR in a stack.
3. For each PR: create the branch with `scripts/stack.sh new <n> <slug>`, implement,
   validate (tests/lint/build), commit atomically, and open it with `scripts/stack.sh pr <n>`.
4. Only move to the next stack level after the previous commit is clean and validated.
5. If blocked on a task, document why and continue with another; do not stop silently.
6. When everything is done and no work remains, report DONE with a summary of the stack.

## Implementation

- Prefer small, focused changes.
- Keep the CLI portable across supported agent environments.
- Validate JSON manifests and generated files before committing.
- Do not include credentials, private study materials, or local agent configuration in the repository.
