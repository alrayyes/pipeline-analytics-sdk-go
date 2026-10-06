#!/usr/bin/env bash
# Assembles the test and coverage reports the CI `pages-deploy` job publishes
# at apis.ryankes.eu/pipeline-analytics-sdk-go/reports/.
#
# Inputs, in the working directory: coverage.out (go test -coverprofile) and
# junit.xml (gotestsum --junitfile). Output: <dest>/reports/, ready to upload
# as the Pages artifact. This repo's Pages root is already
# apis.ryankes.eu/pipeline-analytics-sdk-go/, so <dest> is the artifact root and
# takes no repo-name prefix.
set -euo pipefail

dest="${1:?usage: build-reports.sh <dest-dir>}"
reports="$dest/reports"

mkdir -p "$reports/tests" "$reports/coverage"

cp junit.xml "$reports/tests/junit.xml"
cp coverage.out "$reports/coverage/coverage.out"

# Cobertura XML and the HTML view come from the pinned Go image, like every
# other Go command here, so the host needs only Docker.
./scripts/go-docker.sh sh -c "
  go run github.com/boumenot/gocover-cobertura@v1.4.0 < coverage.out > coverage.xml &&
  go tool cover -html=coverage.out -o coverage.html
"
mv coverage.xml "$reports/coverage/coverage.xml"
mv coverage.html "$reports/coverage/index.html"

cat > "$reports/tests/index.html" <<'HTML'
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Test results</title></head>
<body><main><h1>Test results</h1>
<ul><li><a href="junit.xml">junit.xml</a> (JUnit XML from gotestsum)</li></ul>
<p><a href="../">All reports</a></p></main></body>
</html>
HTML

cat > "$reports/index.html" <<'HTML'
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>pipeline-analytics-sdk-go reports</title></head>
<body><main><h1>pipeline-analytics-sdk-go reports</h1>
<ul>
<li><a href="tests/">Test results</a> (JUnit XML)</li>
<li><a href="coverage/">Coverage</a> (<a href="coverage/coverage.xml">Cobertura XML</a>, <a href="coverage/coverage.out">Go profile</a>)</li>
</ul></main></body>
</html>
HTML
