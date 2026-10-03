#!/bin/sh
set -eu

if [ "${1:-test}" = archives ]; then
  cd /archives
  grep -v "hostlens-${HOSTLENS_VERSION}-tools.json" checksums.txt >/tmp/hostlens-archive-checksums
  sha256sum --check --strict /tmp/hostlens-archive-checksums
  python3 -B /source/tools/release/verify.py /archives "${HOSTLENS_VERSION}"
  exit 0
fi

mkdir /tmp/work
cp -R /source/go.mod /source/go.sum /source/cmd /source/internal \
  /source/packaging /source/scripts /source/tools /source/docs /source/LICENSE /source/NOTICE /tmp/work/
cd /tmp/work

go version
printf '%s\n' 'HOSTLENS_STAGE: module integrity'
go mod verify

if [ "${1:-test}" = benchmark ]; then
  printf '%s\n' 'HOSTLENS_STAGE: bounded MCP benchmark'
  go test -run '^$' -bench 'BenchmarkGateway' -benchmem -benchtime=200ms -count=5 ./internal/server
  printf '%s\n' 'PASS: benchmark completed'
  exit 0
fi

printf '%s\n' 'HOSTLENS_STAGE: race tests'
if [ "${1:-test}" = coverage ]; then
  mkdir /tmp/coverage-data
  go test -race -count=1 -timeout=180s -coverpkg=./internal/... ./... -args -test.gocoverdir=/tmp/coverage-data
  go tool covdata textfmt -i=/tmp/coverage-data -pkg=github.com/Monska85/hostlens/internal/... -o=/coverage/coverage.out
  go tool cover -func=/coverage/coverage.out >/coverage/functions.txt
  go tool cover -html=/coverage/coverage.out -o /coverage/index.html
  tail -n 1 /coverage/functions.txt
else
  go test -race -count=1 -timeout=180s ./...
fi
printf '%s\n' 'HOSTLENS_STAGE: vet'
go vet ./...
printf '%s\n' 'HOSTLENS_STAGE: Linux amd64 and arm64 builds'
for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" go build -trimpath -o "/tmp/hostlens-${arch}" ./cmd/hostlens
done
printf '%s\n' 'HOSTLENS_STAGE: portable interface builds'
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/hostlens-darwin-arm64 ./cmd/hostlens
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o /tmp/hostlens-windows-amd64.exe ./cmd/hostlens
printf '%s\n' 'HOSTLENS_STAGE: generated MCP contract'
go run ./tools/contract >/tmp/tools.json
cmp /tmp/tools.json docs/current/tools.json
printf '%s\n' 'HOSTLENS_STAGE: source formatting'
test -z "$(gofmt -l cmd internal tools)"
python3 -B -m compileall -q tools/release
python3 -B -m unittest discover -s tools/release -p 'test_*.py'
printf '%s\n' 'PASS: container validation completed'
