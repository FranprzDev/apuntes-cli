#!/usr/bin/env bash
# Helper para stacked PRs. Uso:
#   scripts/stack.sh new <n> <slug> [base]   crea rama stack/<n>-<slug> sobre base (default: main o la stack anterior)
#   scripts/stack.sh pr <n>                  abre el PR de stack/<n>-* contra su base
set -euo pipefail

cmd="${1:-}"; shift || true

prev_branch() {
  local n="$1"
  if [ "$n" -le 1 ]; then echo "main"; return; fi
  local matches
  matches=$( { git for-each-ref --format='%(refname:short)' "refs/heads/stack/$((n-1))-*"; git for-each-ref --format='%(refname:short)' "refs/remotes/origin/stack/$((n-1))-*" | sed 's|^origin/||'; } | sort -u )
  local count
  count=$(printf '%s' "$matches" | grep -c . || true)
  if [ "$count" -eq 1 ]; then
    echo "$matches"
  else
    echo "ERROR: se esperaba exactamente una rama stack/$((n-1))-* como base (hay $count)" >&2
    return 1
  fi
}

current_stack_n() {
  git branch --show-current | sed -n 's|^stack/\([0-9]\+\)-.*|\1|p'
}

case "$cmd" in
  new)
    n="$1"; slug="$2"
    base=$(prev_branch "$n")
    git checkout -b "stack/$n-$slug" "$base"
    echo "Rama stack/$n-$slug creada sobre $base"
    ;;
  pr)
    n="$1"
    branch=$(git branch --show-current)
    if [ "$(current_stack_n)" != "$n" ]; then
      echo "ERROR: la rama actual ($branch) no corresponde a stack/$n" >&2
      exit 1
    fi
    base=$(prev_branch "$n")
    gh pr create --base "$base" --head "$branch" --title "${2:-$branch}" --fill
    ;;
  *)
    grep '^#' "$0" | head -5; exit 1 ;;
esac
