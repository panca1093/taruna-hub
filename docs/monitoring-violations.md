# Monitoring and violations (P19)

How tryout violations are recorded and how staff see active sessions. Contract: `api/openapi.yaml` (tag
`monitoring`). Code: `service/monitoring.go`, `repository/monitoring.go`, `handler/monitoring.go`, migration
`000016`. Tryout only; Kecermatan has no violations and no fullscreen enforcement.

## What is recorded

`tryout_session_violations` holds append-only evidence: `exit_fullscreen` and `screenshot_attempt`, with the
subtest that was running and `occurred_at` from the **server clock** (the browser never sends a time). Violations
are evidence only: the timer keeps running and nothing is auto-submitted, penalized or scored.

`POST /my/tryout-sessions/{id}/violations` `{event_id, type}` (Student, own session):

- accepted only while the session is in progress and a subtest is running (`409 session_completed` /
  `409 subtest_not_running` otherwise); another student's or an unknown id is `404`;
- `event_id` is the browser's retry identity (unique per session): a resend returns `200` with the stored row, a new
  event `201`;
- a session holds at most 100 violations; the next new event is `429 too_many_requests`. A resend of a stored
  event is still acknowledged at the cap.

Deadlines settle first (same routine as answers), so a report after the cutoff is refused and never stored.

## Staff views (Admin and Operator)

- `GET /admin/monitoring/sessions` lists in-progress sessions (student, tryout, running subtest, started at,
  violation count), newest first, read from PostgreSQL on every call. The page polls every 30 seconds and keeps the
  last good rows if a poll fails. A session disappears when it completes.
- `GET /admin/tryout-sessions/{id}/violations` is the timeline (type, subtest, time, oldest first, ties by insertion
  order) and works for finished sessions. The admin results detail and the monitoring detail both use it.

Indexes: `tryout_sessions_monitor_idx` (in-progress sessions per tenant) and
`tryout_session_violations_timeline_idx` (per-session timeline and the per-row count).

## Browser behavior (best-effort)

The subtest page asks for fullscreen when it loads.

| Situation | Behavior |
|---|---|
| Fullscreen entered, then left | warning overlay with a return button, `exit_fullscreen` recorded, timer unaffected |
| Browser refuses the automatic request (no user gesture) | the same overlay offers the button; **nothing is recorded** because the student never left fullscreen |
| No Fullscreen API (e.g. iPhone Safari) | the student takes the tryout normally, no overlay, no violation (owner decision 2026-10-28) |
| PrintScreen, Cmd+Shift+3/4/5, Win+Shift+S | default prevented where the browser allows it, warning popup, `screenshot_attempt` recorded |

OS-level capture cannot be reliably blocked from a page, and some shortcuts are never delivered to the page, so
screenshot detection is a deterrent and evidence source, not enforcement. The browser reports with fire-and-forget
retries (same `event_id`, up to 4 attempts on network/5xx failures); a lost report is possible if the device is
offline for the whole window.

## Polling fixture

`TestMonitoringPollingFiftyActiveSessions` (`service/monitoring_test.go`) builds 50 in-progress sessions with 1-4
violations each against a real PostgreSQL, polls `ActiveSessions` 200 times and fails if p95 exceeds 500ms. It
measures the service and query, not HTTP or network. Run: `cd api && go test ./internal/service -run Polling -v`.
