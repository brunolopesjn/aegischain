#!/usr/bin/env bash
set -euo pipefail

ORTHANC_URL="${ORTHANC_URL:-http://localhost:8042}"
ORTHANC_USER="${ORTHANC_USER:-orthanc}"
ORTHANC_PASS="${ORTHANC_PASS:-orthanc}"

if [ "$#" -eq 0 ]; then
  echo "Uso: $0 <pasta1> [pasta2] [pasta3] ..."
  echo "Cada pasta deve conter (recursivamente) arquivos .dcm."
  exit 1
fi

total=0
success=0
failed=0

for dir in "$@"; do
  echo "== Importando de: $dir =="
  while IFS= read -r -d '' file; do
    total=$((total + 1))
    status=$(curl -s -o /dev/null -w "%{http_code}" \
      -X POST "$ORTHANC_URL/instances" \
      --data-binary @"$file" \
      -u "$ORTHANC_USER:$ORTHANC_PASS")

    if [ "$status" = "200" ]; then
      success=$((success + 1))
    else
      failed=$((failed + 1))
      echo "  FALHOU ($status): $file"
    fi
  done < <(find "$dir" -type f -name "*.dcm" ! -name "._*" -print0)
done

echo ""
echo "== Resumo =="
echo "Total: $total | Sucesso: $success | Falhas: $failed"