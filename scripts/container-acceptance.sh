#!/bin/sh
set -eu

# Run only inside the disposable tool container created by test-container.sh.
if [ "${1:-test}" = archives ]; then
  cd /source
  go build -o /tmp/archive ./tools/archive
  cd /archives
  sha256sum "hostlens-${HOSTLENS_VERSION}-linux-amd64.tar.gz" "hostlens-${HOSTLENS_VERSION}-linux-arm64.tar.gz" | sort >/tmp/hostlens-expected-checksums
  sort checksums.txt | diff /tmp/hostlens-expected-checksums -
  for arch in amd64 arm64; do
    /tmp/archive -archive "hostlens-${HOSTLENS_VERSION}-linux-${arch}.tar.gz" -arch "${arch}" -version "${HOSTLENS_VERSION}"
  done
  exit 0
fi
mkdir /tmp/work
cp -R /source/go.mod /source/go.sum /source/cmd /source/internal \
  /source/.github /source/packaging /source/scripts /source/tools /source/docs /source/LICENSE /source/NOTICE /tmp/work/
cd /tmp/work

go version
cat /etc/os-release
if ! go list -deps ./cmd/... >/dev/null; then
  printf '%s\n' 'Module cache incomplete; run go mod download with the selected GOMODCACHE before retrying.' >&2
  exit 1
fi

printf '%s\n' 'HOSTLENS_STAGE: validate module integrity'
go mod verify
printf '%s\n' 'HOSTLENS_STAGE: run go race tests'
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
printf '%s\n' 'HOSTLENS_STAGE: run go vet'
go vet ./...
printf '%s\n' 'HOSTLENS_STAGE: build linux candidates'
for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" go build -trimpath \
    -o "/tmp/build/${arch}/" ./cmd/...
done
printf '%s\n' 'HOSTLENS_STAGE: check portable build boundaries'
# Shared protocol, policy and observer contracts must remain free of native runtime APIs.
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./internal/contract ./internal/config ./internal/policy ./internal/token ./internal/backend ./internal/gateway ./internal/dockerobs
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./internal/contract ./internal/config ./internal/policy ./internal/token ./internal/backend ./internal/gateway ./internal/dockerobs
test -z "$(gofmt -l cmd internal tools)"

printf '%s\n' 'HOSTLENS_STAGE: test release utilities'
python3 -B -m unittest discover -s tools/release -p 'test_*.py'
printf '%s\n' 'HOSTLENS_STAGE: test matrix orchestration'
python3 -B -m unittest discover -s tools/test-matrix -p 'test_*.py'
printf '%s\n' 'HOSTLENS_STAGE: test developer commands'
python3 -B -m unittest discover -s tools/dev -p 'test_*.py'
printf '%s\n' 'PASS: container validation completed'
