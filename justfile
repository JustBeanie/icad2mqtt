set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]

check:
	go test ./...
	go vet ./...
	$files = git ls-files '*.go' | Where-Object { Test-Path $_ }; if ($files) { $bad = gofmt -l $files; if ($bad) { $bad; exit 1 } }
	go build ./...

test-race:
	@echo "Race tests are CI/Linux-only; run: go test -race ./..."
