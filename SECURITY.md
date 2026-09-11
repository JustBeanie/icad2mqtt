# Security policy

## Supported versions

Only the latest commit on `master` is supported.

## Reporting a vulnerability

Please do not open a public issue for a suspected security vulnerability. Use
GitHub's private vulnerability reporting for this repository, or contact the
maintainer privately with reproduction details, affected versions, and impact.

Do not include live broker credentials or personally identifiable call data in a
report. Redact those values before sending logs or payloads.

## Security expectations

- Keep MQTT credentials out of environment files committed to Git.
- Run the container as the supplied non-root user.
- Review dependency and image scan findings before releasing an image.
- Treat the upstream CAD response as untrusted external input.
