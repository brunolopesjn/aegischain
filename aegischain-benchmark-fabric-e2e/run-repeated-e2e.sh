#!/bin/bash
N=${1:-14}
for i in $(seq 1 "$N"); do
  echo ""
  echo "############################################"
  echo "## Repetição $i/$N"
  echo "############################################"
  ./aegischain-benchmark-fabric-e2e -skip-sequential -timeout 90 | tee "e2e-run-$i.log"
done
