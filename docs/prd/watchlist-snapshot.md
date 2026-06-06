# Watchlist Snapshot Export

## Problem Statement

Automation Engineers use the OPC UA Client TUI during a Troubleshooting Session to inspect Variable Nodes and collect a Watchlist of Live Values. Today, they can see those values in the TUI, but they cannot capture the current Watchlist state as a durable Snapshot to share with a colleague, attach to a ticket, or compare against later observations.

The v1 plan explicitly includes Watchlist Snapshots, and the help overlay already advertises an `s` key for exporting a Snapshot. That advertised action should become real, predictable, and safe enough for troubleshooting use.

## Solution

Add a Watchlist Snapshot export action to the OPC UA Client TUI. When an Automation Engineer presses `s`, the app writes a timestamped markdown Snapshot of the current Watchlist to local app storage and updates the status line with either the export path or a clear failure message.

The Snapshot should include the current Troubleshooting Session context and each watched Variable Node's current inspection state: DisplayName, NodeId, Live Value, OPC UA StatusCode, timestamps, Stale Value state, and Out-of-Range state when present. Because node names, process values, endpoints, and server metadata may be sensitive, the TUI should make the sensitivity of Snapshot exports explicit.

## User Stories

1. As an Automation Engineer, I want to export my current Watchlist as a Snapshot, so that I can preserve what I observed during a Troubleshooting Session.
2. As an Automation Engineer, I want to trigger the Snapshot from the keyboard, so that I can stay in the terminal workflow.
3. As an Automation Engineer, I want the existing `s` help hint to perform the Snapshot export, so that the advertised shortcut behaves as expected.
4. As an Automation Engineer, I want the Snapshot to include each watched Variable Node's DisplayName, so that I can recognize values quickly.
5. As an Automation Engineer, I want the Snapshot to include each watched Variable Node's NodeId, so that I can identify the exact OPC UA node later.
6. As an Automation Engineer, I want the Snapshot to include the current Live Value, so that the Snapshot is useful as a troubleshooting record.
7. As an Automation Engineer, I want the Snapshot to include the OPC UA StatusCode for each Live Value, so that I can distinguish good values from bad or uncertain values.
8. As an Automation Engineer, I want the Snapshot to include source and server timestamps when available, so that I can understand when each value was produced and observed.
9. As an Automation Engineer, I want the Snapshot to show when a Live Value is stale, so that I do not mistake an old value for current server state.
10. As an Automation Engineer, I want the Snapshot to include Out-of-Range state when a value falls outside server-provided range metadata, so that important abnormal readings are preserved.
11. As an Automation Engineer, I want values without Out-of-Range state to avoid noisy warnings, so that the Snapshot remains focused.
12. As an Automation Engineer, I want the Snapshot to include Server Connection context such as endpoint, security mode, security policy, and authentication type, so that I know which OPC UA Server and connection settings produced the observations.
13. As an Automation Engineer, I want secrets excluded from the Snapshot, so that exported troubleshooting material does not leak credentials.
14. As an Automation Engineer, I want the Snapshot export to warn that node names, process values, endpoints, and server metadata may be sensitive, so that I handle the file appropriately.
15. As an Automation Engineer, I want a successful export to show the Snapshot file path, so that I can find it immediately.
16. As an Automation Engineer, I want export failures to be shown clearly in the status line, so that I know the Snapshot was not created.
17. As an Automation Engineer, I want Snapshot files to have timestamped names, so that repeated exports do not overwrite each other.
18. As an Automation Engineer, I want Snapshot files stored in the app's local storage area, so that exports have a predictable location.
19. As an Automation Engineer using portable mode, I want Snapshots stored under the portable app storage location, so that the app remains self-contained.
20. As an Automation Engineer using OS-standard paths, I want Snapshots stored under the normal local app storage location, so that exports do not clutter my working directory.
21. As an Automation Engineer, I want an empty Watchlist export to fail or report clearly that there is nothing to export, so that I do not create misleading empty Snapshots.
22. As an Automation Engineer, I want Snapshot export to work without reconnecting or rebrowsing, so that it captures the state I already collected.
23. As an Automation Engineer, I want Snapshot export to avoid changing the current focus or selected node, so that I can continue troubleshooting after export.
24. As an Automation Engineer, I want Snapshot export to preserve the current Read-Only Mode behavior, so that exporting a Snapshot never writes to the OPC UA Server.
25. As an Automation Engineer, I want the Snapshot format to be human-readable, so that I can open it without special tooling.
26. As an Automation Engineer, I want the Snapshot to be shareable in tickets or chat, so that teammates can understand the observed state.
27. As an Automation Engineer, I want the Snapshot to include enough context to compare with later Snapshots, so that I can reason about changes during a Troubleshooting Session.
28. As an Automation Engineer, I want Snapshot export to include watched nodes even if their latest value is stale or unavailable, so that missing data is visible rather than silently omitted.
29. As an Automation Engineer, I want Snapshot export to avoid storing passwords, so that username/password Server Connections remain safe.
30. As an Automation Engineer, I want Snapshot export to behave consistently for Anonymous and username/password Server Connections, so that the feature works across supported authentication modes.

## Implementation Decisions

- The Snapshot export is a Read-Only Mode feature. It exports local troubleshooting information and must not write to the OPC UA Server or call OPC UA methods.
- The Snapshot source of truth is the current Watchlist inspection state, not a fresh OPC UA read. Exporting captures what the TUI currently knows.
- Snapshot content should be markdown for v1 because it is human-readable, easy to attach to tickets, and simple to test.
- Snapshot files should use timestamped filenames to avoid overwriting previous exports.
- Snapshot files should be written under the app's configured local storage paths, respecting portable mode path resolution.
- The TUI should handle `s` as the Snapshot export shortcut, matching the existing help overlay.
- The TUI should update the status line on success with the exported file path.
- The TUI should update the status line on failure with a clear error message.
- If the Watchlist is empty, the TUI should not create a misleading Snapshot; it should report that there are no watched Variable Nodes to export.
- The Snapshot should include Server Connection context: endpoint, security mode, security policy, and authentication type.
- The Snapshot must not include passwords or other secrets.
- The Snapshot should include a sensitivity warning stating that node names, process values, endpoints, and server metadata may be sensitive.
- Each watched Variable Node entry should include DisplayName, NodeId, Live Value, OPC UA StatusCode, source timestamp, server timestamp, Stale Value state, and Out-of-Range state when present.
- Out-of-Range should use the existing inspection state already computed from server-provided range metadata and numeric Live Values.
- Stale Value should use the existing inspection state rather than inventing a second stale-value calculation.
- Exporting should not alter focus, selection, Watchlist order, Server Connection state, or subscription state.
- Introduce a small snapshot export boundary so the TUI can request an export without directly owning filesystem formatting details.
- The boundary should be fakeable in model tests and backed by filesystem writing in production.
- The feature should be implemented as a bounded v1 Snapshot export, not as the broader Diagnostics Bundle export.

## Testing Decisions

- Tests should assert external behavior: user-visible status changes, produced Snapshot content, and file creation behavior. They should avoid coupling to private helper structure.
- The highest-value tests should exercise the TUI model seam by simulating the `s` key with watched Variable Nodes and checking success/failure status behavior.
- Existing keypress-driven TUI model tests provide prior art for this seam, especially tests around adding Variable Nodes to the Watchlist and rendering Watchlist state.
- Session-state tests should rely on the existing inspection model as the source of watched Variable Nodes, Live Values, Stale Value, and Out-of-Range state.
- Existing inspection tests provide prior art for Stale Value and Out-of-Range state.
- Snapshot formatting tests should use a fake or in-memory writer where possible, so most tests do not touch the real filesystem.
- Snapshot content tests should verify that markdown includes watched Variable Nodes, Live Values, OPC UA StatusCode, timestamps, Stale Value, and Out-of-Range when present.
- Snapshot content tests should verify that secrets such as passwords are not emitted.
- Empty Watchlist behavior should be tested from the TUI model seam.
- Filesystem behavior should be covered by one focused test that verifies timestamped Snapshot creation under configured local storage.
- Portable-mode path behavior can be covered through existing path-resolution seams rather than duplicating OS-specific path tests.
- Tests should not require a live OPC UA Server.

## Out of Scope

- Diagnostics Bundle export.
- In-memory session diagnostic log capture.
- Search or Address Space indexing.
- Saved Connections.
- Server certificate trust management.
- Auto-reconnect and subscription restoration.
- Session Trend views.
- Copying NodeIds to the clipboard.
- Writing to Variable Nodes.
- OPC UA method calls.
- Additional export formats such as JSON or CSV.
- A full export configuration UI.
- Redaction rules beyond excluding known secrets and warning about sensitive troubleshooting content.

## Further Notes

The glossary already defines Snapshot as a point-in-time capture of selected troubleshooting information, such as Watchlist Live Values and node details. This feature should use that term consistently and avoid calling the export a report or dump.

The codebase already supports Watchlist subscriptions, Stale Value state, and Out-of-Range state for selected Variable Node inspection. The Snapshot should reuse those states rather than re-reading values during export.

The issue could not be published from this checkout because no git remote is configured for the repository. Apply the `ready-for-agent` label when creating the tracker issue.
