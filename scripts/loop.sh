#!/usr/bin/env bash
# Loop de trabajo autónomo: relanza opencode hasta que responda DONE.
# Uso: ./scripts/loop.sh [máx iteraciones] [mensaje extra]
set -u

MAX="${1:-20}"
EXTRA="${2:-}"
i=0

while [ "$i" -lt "$MAX" ]; do
  i=$((i + 1))
  echo "=== Iteración $i/$MAX ==="
  out=$(opencode run "/loop $EXTRA")
  echo "$out"
  if printf '%s' "$out" | grep -q '^DONE$'; then
    echo "Terminado: el agente reportó DONE."
    exit 0
  fi
done

echo "Se alcanzó el máximo de $MAX iteraciones."
exit 1
