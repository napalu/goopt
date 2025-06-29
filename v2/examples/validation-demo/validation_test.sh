#!/bin/bash
set -euo pipefail

BIN="./validation-demo"
SRC="main.go"

echo "🔨 Building validation-demo..."
go build -o "$BIN" "$SRC"

PASS=0
FAIL=0

run_test() {
  local desc="$1"
  local args="$2"
  local expected_exit="$3"

  echo -e "\n🔹 $desc"
  echo "   Command: $BIN $args"

  set +e
  $BIN $args > /dev/null 2>&1
  local exit_code=$?
  set -e

  if [[ "$exit_code" -eq "$expected_exit" ]]; then
    echo "   ✅ Success (exit $exit_code)"
    PASS=$((PASS + 1))
  else
    echo "   ❌ Failed (expected $expected_exit, got $exit_code)"
    FAIL=$((FAIL + 1))
  fi
}

# Tests
run_test "Valid email and username" \
  "--email test@example.com --username johndoe" 0

run_test "Invalid email format" \
  "--email not-an-email --username johndoe" 1

run_test "Missing required username" \
  "--email test@example.com" 1

run_test "Invalid port (too high)" \
  "--email test@example.com --username johndoe --port 70000" 1

run_test "Invalid port (non-numeric)" \
  "--email test@example.com --username johndoe --port abc" 1

run_test "Valid webhook URL" \
  "--email test@example.com --username johndoe --webhook https://ok.com" 0

run_test "Invalid webhook scheme" \
  "--email test@example.com --username johndoe --webhook ftp://nope.com" 1

run_test "Valid config file" \
  "--email test@example.com --username johndoe --config config.yaml" 0

run_test "Invalid config extension" \
  "--email test@example.com --username johndoe --config bad.ini" 1

# Summary
echo -e "\n🔍 Test Summary"
echo "✅ Passed: $PASS"
echo "❌ Failed: $FAIL"

if [[ "$FAIL" -gt 0 ]]; then
  echo -e "\n💥 Some tests failed."
  exit 1
else
  echo -e "\n🎉 All tests passed!"
fi
