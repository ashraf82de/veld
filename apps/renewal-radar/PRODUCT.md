# Renewal Radar product brief

## Problem and intended users

Small agencies, independent consultants and small operations teams often keep
renewal dates for client licenses, domains, support contracts, subscriptions,
certificates and keys in spreadsheets, inboxes or individual memory. Missing a
date can interrupt a service or continue an unwanted cost. Users need one
small, auditable inventory without adopting a full procurement platform.

Public problem reports support the shape of the need:

- A self-hosting request asks for a focused tracker for customer subscriptions,
  license counts and renewal dates without invoicing:
  <https://www.reddit.com/r/selfhosted/comments/sy3x3b/>
- Another request describes purchased licenses across clients and the risk of
  forgetting them when a client contract ends:
  <https://www.reddit.com/r/selfhosted/comments/1m36z2e/license_renewal_tracking_app/>
- TokenTimer documents the broader expiring-asset category: certificates, API
  keys, secrets, licenses and contracts:
  <https://tokentimer.ch/docs/self-hosted/0.15>

These sources establish recurring demand and existing competition. They do not
establish adoption for Renewal Radar.

## Candidate comparison

| Candidate | User value | Fit with current Veld | Maintenance cost | Decision |
|---|---|---|---|---|
| Renewal and expiry tracker | Clear deadline risk; useful for one person or a small team | Strong: dates, JSON, files, CLI and HTTP are available | Moderate | Selected |
| Shift handoff log | Prevents context loss between operators | Partial: concurrent durable writes need a stronger storage design | High | Revisit after persistence work |
| General issue tracker | Familiar workflow | Technically feasible, but auth, search and integrations expand scope | High and crowded | Rejected |

## Product principles

- Local and self-hostable: users own a portable data file.
- Focused: track renewal ownership and dates rather than procurement or billing.
- Auditable: explicit validation, deterministic ordering and recoverable writes.
- Useful without a server: the CLI is a complete workflow, not a demo fixture.
- Honest evidence: releases, usability trials and reports are recorded; generated
  examples and passing reference tests are not counted as adoption.

## First-slice acceptance criteria

- Add a named renewal with a validated expiry date and optional owner,
  category and notes.
- List items in expiry order with overdue and upcoming status, and view all
  details for one stable ID.
- Filter items due within a configurable number of days.
- Import a five-column CSV only after all records and duplicates validate, with
  a dry-run mode that performs no write.
- Renew or remove an item by stable integer ID.
- Persist a portable JSON file and retain a previous-file backup on writes.
- Reject malformed persisted data instead of silently discarding it.
- Include regression tests for dates, status boundaries, JSON and ordering.

## Known limitations

- The current CLI supports one writer at a time.
- It does not send notifications, authenticate users or encrypt sensitive
  content. Store only metadata suitable for a local plaintext file.
- A browser interface, JSON export and notification adapters remain roadmap
  work and need their own verification.
