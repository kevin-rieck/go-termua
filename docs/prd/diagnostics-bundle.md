# Diagnostics Bundle Export

## Status

Implemented on the `feature/diagnostics-bundle-export` branch. The `d` key exports a timestamped markdown Diagnostics Bundle through a fakeable exporter boundary, recent diagnostic events are retained in a bounded in-memory log, and `o` opens the exports folder for the next user action after export.

## Problem Statement

Automation Engineers use the OPC UA Client TUI during a Troubleshooting Session against existing OPC UA Servers. When the OPC UA Client TUI itself behaves unexpectedly, fails to connect, loses subscriptions, or shows confusing state, the Automation Engineer has no complete, shareable view of the session diagnostics.

The v1 plan calls for a Diagnostics Bundle containing recent diagnostic logs and session/app diagnostic state. The help overlay already advertises a `d` key for exporting a Diagnostics Bundle, but that action is not implemented. Without it, troubleshooting the client requires ad hoc screenshots, manually copied status text, or external debug logs that may not be enabled.

## Solution

Add a Diagnostics Bundle export action to the OPC UA Client TUI. When an Automation Engineer presses `d`, the app writes a timestamped markdown Diagnostics Bundle to local app storage and updates the status line with either the export path or a clear failure message.

The Diagnostics Bundle should capture the current Troubleshooting Session state and recent in-memory diagnostic log entries. It should include Server Connection context, selected Variable Node context, Watchlist and subscription summary, last visible error or status, local path summary, and recent diagnostic events. Because endpoints, node names, server metadata, and process values may be sensitive, the bundle should include an explicit sensitivity warning and must exclude secrets.

## User Stories

1. As an Automation Engineer, I want to export a Diagnostics Bundle, so that I can troubleshoot problems with the OPC UA Client TUI itself.
2. As an Automation Engineer, I want to trigger the Diagnostics Bundle from the keyboard, so that I can stay in the terminal workflow.
3. As an Automation Engineer, I want the existing `d` help hint to perform the Diagnostics Bundle export, so that the advertised shortcut behaves as expected.
4. As an Automation Engineer, I want the Diagnostics Bundle to include the current Server Connection endpoint, so that I know which OPC UA Server the session was using.
5. As an Automation Engineer, I want the Diagnostics Bundle to include security mode and security policy, so that connection behavior can be diagnosed accurately.
6. As an Automation Engineer, I want the Diagnostics Bundle to include authentication type without secrets, so that support can understand how the Server Connection was made safely.
7. As an Automation Engineer, I want the Diagnostics Bundle to exclude passwords, so that I can share it without leaking credentials.
8. As an Automation Engineer, I want the Diagnostics Bundle to include Server Connection state, so that failures can be understood in context.
9. As an Automation Engineer, I want the Diagnostics Bundle to include the last visible status or error, so that the exact failure symptom is preserved.
10. As an Automation Engineer, I want the Diagnostics Bundle to include the selected Variable Node, so that support can understand what I was inspecting.
11. As an Automation Engineer, I want the Diagnostics Bundle to include Watchlist count, so that subscription and session scale are visible.
12. As an Automation Engineer, I want the Diagnostics Bundle to include subscription summary information, so that subscription failures can be diagnosed.
13. As an Automation Engineer, I want the Diagnostics Bundle to identify stale watched values, so that disconnected or closed subscriptions are visible.
14. As an Automation Engineer, I want the Diagnostics Bundle to include recent diagnostic log entries, so that failures leading up to the export can be investigated.
15. As an Automation Engineer, I want diagnostic logs retained in memory during the Troubleshooting Session, so that diagnostics are available even when debug file logging was not enabled.
16. As an Automation Engineer, I want the in-memory diagnostic log to be bounded, so that a long Troubleshooting Session does not consume unbounded memory.
17. As an Automation Engineer, I want the Diagnostics Bundle to include local config/cache/export path information, so that storage and portable-mode issues can be diagnosed.
18. As an Automation Engineer using portable mode, I want the Diagnostics Bundle written under portable app storage, so that the app remains self-contained.
19. As an Automation Engineer using OS-standard paths, I want the Diagnostics Bundle written under the normal local app storage location, so that files are predictable and do not clutter my working directory.
20. As an Automation Engineer, I want Diagnostics Bundle filenames to be timestamped, so that repeated exports do not overwrite each other.
21. As an Automation Engineer, I want a successful export to show the Diagnostics Bundle path, so that I can find it immediately.
22. As an Automation Engineer, I want export failures to be shown clearly, so that I know the Diagnostics Bundle was not created.
23. As an Automation Engineer, I want exporting diagnostics to avoid changing focus, selection, Watchlist state, Server Connection state, or subscriptions, so that I can continue troubleshooting after export.
24. As an Automation Engineer, I want Diagnostics Bundle export to preserve Read-Only Mode behavior, so that exporting never writes to the OPC UA Server.
25. As an Automation Engineer, I want the Diagnostics Bundle to be human-readable, so that I can inspect it before sharing.
26. As an Automation Engineer, I want the Diagnostics Bundle to warn that endpoints, node names, process values, and server metadata may be sensitive, so that I handle the file appropriately.
27. As an Automation Engineer, I want the Diagnostics Bundle to be useful even before a successful Server Connection, so that endpoint discovery and connection failures can be diagnosed.
28. As an Automation Engineer, I want the Diagnostics Bundle to be useful after a successful Server Connection, so that browse, selected-value, and Watchlist issues can be diagnosed.
29. As an Automation Engineer, I want the Diagnostics Bundle to include enough app state to compare with screenshots or reported behavior, so that bug reports are easier to reproduce.
30. As an Automation Engineer, I want the Diagnostics Bundle to use the same export conventions as Snapshot export, so that exported troubleshooting files are consistent.

## Implementation Decisions

- The Diagnostics Bundle export is a Read-Only Mode feature. It exports local diagnostic information and must not write to the OPC UA Server or call OPC UA methods.
- The Diagnostics Bundle should be generated from the current TUI/session state and recent in-memory diagnostic events. It should not trigger new OPC UA reads, writes, browses, subscriptions, or reconnects.
- The Diagnostics Bundle should use markdown for v1 because it is human-readable, easy to inspect before sharing, and consistent with the recommended Snapshot export format.
- Diagnostics Bundle files should use timestamped filenames to avoid overwriting previous exports.
- Diagnostics Bundle files should be written under the app's configured local storage paths, respecting portable mode path resolution.
- The TUI should handle `d` as the Diagnostics Bundle export shortcut, matching the existing help overlay.
- The TUI should update the status line on success with the exported file path.
- The TUI should update the status line on failure with a clear error message.
- The Diagnostics Bundle should include a sensitivity warning stating that endpoints, node names, process values, and server metadata may be sensitive.
- The Diagnostics Bundle must not include passwords or other secrets.
- The Diagnostics Bundle should include Server Connection context: endpoint, security mode, security policy, authentication type, connection state, and latest connection/discovery error when known.
- The Diagnostics Bundle should include selected Variable Node context when a Variable Node is selected: DisplayName, NodeId, NodeClass, current Live Value status summary, Stale Value state, and Out-of-Range state when present.
- The Diagnostics Bundle should include Watchlist summary: watched Variable Node count and per-node subscription state at a summary level.
- The Diagnostics Bundle should include subscription count or the best available app-level approximation from current inspection state.
- The Diagnostics Bundle should include local config/cache/export path summary.
- The Diagnostics Bundle should include recent diagnostic log entries collected in memory during the current Troubleshooting Session.
- Add a bounded in-memory diagnostic log for session diagnostics. It should retain recent events and evict older entries when full.
- Existing logging can continue, but Diagnostics Bundle export should not depend on debug file logging being enabled.
- Introduce a small diagnostics export boundary so the TUI can request an export without directly owning filesystem formatting details.
- The diagnostics export boundary should be fakeable in TUI model tests and backed by filesystem writing in production.
- Exporting should not alter focus, selected node, Watchlist order, Server Connection state, subscription state, or visible pane selection.
- This slice should build on the same export conventions as Watchlist Snapshot export if that feature exists: timestamped files, local app storage, markdown format, sensitivity warning, and status-line result.
- The feature should be implemented as Diagnostics Bundle export only, not as a general-purpose logging, telemetry, or support-upload system.

## Testing Decisions

- Tests should assert external behavior: pressing `d`, status-line updates, produced Diagnostics Bundle content, secret exclusion, and file creation behavior. They should avoid coupling to private helper structure.
- The highest-value tests should exercise the TUI model seam by simulating the `d` key and checking success/failure status behavior.
- Existing keypress-driven TUI model tests provide prior art for this seam, especially tests around help text, connection flow, selected Variable Node inspection, and Watchlist behavior.
- TUI model tests should verify that Diagnostics Bundle export does not change focus, selection, Watchlist state, Server Connection state, or Read-Only Mode UI state.
- Diagnostics collection tests should use fake or constructed session state to verify the bundle includes endpoint, security mode/policy, auth type, connection state, selected Variable Node context, Watchlist/subscription summary, last error/status, paths, and recent logs.
- Diagnostics content tests should verify that passwords and known secrets are not emitted.
- In-memory diagnostic log tests should verify that recent events are retained, ordering is stable, and older events are evicted when the bounded capacity is exceeded.
- Filesystem behavior should be covered by one focused test that verifies timestamped Diagnostics Bundle creation under configured local storage.
- Most tests should avoid the real filesystem by using a fake or in-memory export boundary.
- Tests should not require a live OPC UA Server.
- Tests should treat the Diagnostics Bundle as user-facing output. They should assert important content and omissions without over-specifying exact whitespace or incidental formatting.

## Out of Scope

- Watchlist Snapshot export, except for reusing any export conventions or infrastructure if already implemented.
- Search or Address Space indexing diagnostics beyond reporting current known state if available.
- Saved Connections.
- Server certificate trust management.
- Client certificate/key configuration.
- Auto-reconnect and subscription restoration.
- Session Trend views.
- Copying NodeIds to the clipboard.
- Writing to Variable Nodes.
- OPC UA method calls.
- Remote upload of diagnostics.
- Zip archive generation.
- Full structured telemetry.
- Persistent log storage across app launches.
- Redaction rules beyond excluding known secrets and warning about sensitive troubleshooting content.

## Further Notes

The glossary defines Diagnostics Bundle as a user-triggered export of information useful for troubleshooting the OPC UA Client TUI itself, such as connection state, recent errors, and internal diagnostic logs. This feature should use that term consistently and avoid calling it a report, dump, telemetry package, or support upload.

The v1 plan says the app keeps logs in memory during a Troubleshooting Session and allows exporting a Diagnostics Bundle containing recent diagnostic logs and session/app diagnostic state. Current code logs through standard logging and optional debug-file logging, so this slice needs a bounded in-memory diagnostic log to satisfy the plan without depending on debug mode.

The issue could not be published from this checkout because no git remote is configured for the repository. Apply the `ready-for-agent` label when creating the tracker issue.
