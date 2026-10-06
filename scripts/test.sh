#!/usr/bin/env bash
# Quick checks after "make deploy".
set -uo pipefail

NS=ginflix
failed=0

ok()   { echo "  [OK]   $1"; }
fail() { echo "  [FAIL] $1"; failed=1; }

# runs a command in a temporary busybox pod and prints its output
# (long timeout: the first time, the node has to download busybox)
in_pod() {
  kubectl run "test-$RANDOM" -n "$NS" --rm -i --quiet --restart=Never \
    --pod-running-timeout=5m --image=busybox:1.36 -- sh -c "$1" 2>/dev/null
}

echo "1. workloads are ready"
for d in backend streamer frontend frontend-admin keycloak seaweedfs; do
  if kubectl rollout status "deployment/$d" -n "$NS" --timeout=120s > /dev/null 2>&1; then
    ok "deployment $d"
  else
    fail "deployment $d"
  fi
done
if kubectl rollout status statefulset/mongodb -n "$NS" --timeout=120s > /dev/null 2>&1; then
  ok "statefulset mongodb"
else
  fail "statefulset mongodb"
fi

echo "2. MongoDB replica set has a primary"
primary=$(kubectl exec -n "$NS" mongodb-0 -- mongosh --quiet --eval \
  'rs.status().members.filter(m => m.stateStr == "PRIMARY").map(m => m.name).join()' 2>/dev/null)
if [ -n "$primary" ]; then
  ok "primary is $primary"
else
  fail "no primary (check: kubectl logs -n $NS job/mongo-init)"
fi

echo "3. every service has pods behind it"
for s in backend streamer frontend frontend-admin keycloak seaweedfs; do
  ips=$(kubectl get endpoints "$s" -n "$NS" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null)
  if [ -n "$ips" ]; then
    ok "service $s -> $ips"
  else
    fail "service $s has no endpoints"
  fi
done

echo "4. backend answers (and can read MongoDB)"
out=$(in_pod "wget -q -T 10 -O /dev/null http://backend:8080/api/videos && echo API_OK")
if echo "$out" | grep -q API_OK; then
  ok "GET /api/videos"
else
  fail "GET /api/videos"
fi

echo "5. Keycloak gives a token to the demo user"
out=$(in_pod "wget -q -T 10 -O - --post-data 'grant_type=password&client_id=ginflix&username=ginflix&password=ginflix' http://keycloak:8080/realms/ginflix/protocol/openid-connect/token")
if echo "$out" | grep -q access_token; then
  ok "login ginflix / ginflix"
else
  fail "no token from Keycloak"
fi

echo "6. network policies: a random pod must NOT reach MongoDB and the S3 storage"
check_blocked() {
  out=$(in_pod "nc -z -w 3 $2 $3 && echo OPEN || echo BLOCKED")
  if echo "$out" | grep -q BLOCKED; then
    ok "$1 is not reachable from a random pod"
  elif echo "$out" | grep -q OPEN; then
    fail "$1 is reachable: the CNI does not enforce NetworkPolicy"
  else
    fail "the test pod did not run"
  fi
}
check_blocked MongoDB   mongodb-0.mongodb 27017
check_blocked SeaweedFS seaweedfs 8333

echo
if [ "$failed" -eq 0 ]; then
  echo "All checks passed."
else
  echo "Some checks failed."
fi
exit "$failed"
