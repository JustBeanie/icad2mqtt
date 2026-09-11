# OWASP SAMM / DSOMM audit

This is a lightweight, repository-scoped audit for the ICAD to MQTT bridge. It
is not a formal certification. SAMM provides the broader software assurance
model, while DSOMM focuses on security activities integrated into DevOps
workflows.

## Findings addressed in this change

| Area | Finding | Remediation | Evidence |
| --- | --- | --- | --- |
| SAMM Secure Build / DSOMM | The build could not reliably start because the primary Go entrypoint was malformed and the Dockerfile assumed an absent `go.sum`. | Rebuilt the entrypoint, added tests, and made dependency download work from tracked `go.mod`. | `main.go`, `main_test.go`, `Dockerfile` |
| SAMM Secure Deployment | Runtime image used a floating `latest` base and created the runtime user late in the build. | Use a versioned Alpine base and run as a dedicated non-login user. | `Dockerfile` |
| SAMM Secure Architecture | Network failures, non-200 responses, oversized responses, and shutdown were not bounded. | Add request context/timeout, status checks, response-size limit, and SIGTERM handling. | `main.go` |
| SAMM Security Requirements / DSOMM | Configuration errors could silently fall back to an unexpected polling interval. | Validate `POLL_INTERVAL` and fail fast on invalid values. | `main.go`, `main_test.go` |
| SAMM Environment Management | The repository had no automated quality or dependency security gates. | Add format, vet, race tests, golangci-lint, govulncheck, and Trivy image scanning. | `.github/workflows/ci.yml`, `.golangci.yml` |
| SAMM Operational Management | Security reporting and credential-handling expectations were undocumented. | Add a vulnerability reporting policy and operational security notes. | `SECURITY.md` |

## Residual risks and next steps

- MQTT transport security is deployment-configured. Use `ssl://`/`tls://`
  broker URLs and broker authentication in production; TLS certificate and
  credential options should be made explicit if this becomes a multi-tenant
  service.
- The upstream endpoint returns a document that is forwarded as-is. Consumers
  should parse it as untrusted data; this bridge does not render or execute it.
- Pinning image digests and generating signed SBOM/provenance attestations would
  improve supply-chain maturity beyond this repository's current baseline.
- Add an integration test with a disposable MQTT broker before claiming full
  end-to-end coverage.

## Verification gates

The required gates are defined in GitHub Actions: `gofmt`, `go vet`, race-enabled
tests, `golangci-lint`, `govulncheck`, a repository-root Docker build, and a
Trivy scan of that image. A change is not considered release-ready unless all
required workflow jobs pass.
