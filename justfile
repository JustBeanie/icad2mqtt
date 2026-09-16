set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]

check:
	go test ./...
	go vet ./...
	$files = git ls-files '*.go'; if ($files) { $bad = gofmt -l $files; if ($bad) { $bad; exit 1 } }
	go build ./...
	Push-Location icad2mqtt; go vet ./...; if ($LASTEXITCODE -ne 0) { Pop-Location; exit $LASTEXITCODE }; go build -o (Join-Path $env:TEMP 'icad2mqtt-addon.exe'); $code = $LASTEXITCODE; Pop-Location; exit $code

test-race:
	@echo "Race tests are CI/Linux-only; run: go test -race ./..."
