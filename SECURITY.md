# Security

Veld runs programs written by AI agents, so its sandbox matters.

## What counts as a vulnerability

- A program that performs an effect (`io fs net env time rand proc state`) that
  it did not declare, or that `veld run --deny ...` was told to forbid.
- A way to make the checker accept a program that then corrupts memory or
  crashes the interpreter with a Go panic instead of a Veld runtime error.
- Path traversal, command injection or unsafe file writes in `veld fix`,
  `veld new`, `veld fmt`, the module loader or the HTTP server.

## Reporting

Please report privately through GitHub's "Report a vulnerability" button (the
repository's Security tab) rather than a public issue. Include a minimal
`.veld` program (`veld report <file>` prints what we need). You will get a
response within a few days; fixes are prioritised as P0.

## Supported versions

Pre-1.0: only the latest release and `main`.
