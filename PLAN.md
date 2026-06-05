# OPC UA Client TUI Plan

## Working name

- Working product name: **TermUA**
- Working command: `termua`
- Repository/package name is unresolved because the GitHub project name is taken.

## Product focus

- Build an **OPC UA Client TUI** for automation engineers.
- v1 focuses on **Troubleshooting Sessions** against existing OPC UA Servers.
- v1 is **Read-Only Mode**: no writes and no method calls.
- Central workflow: connect → browse Address Space → inspect Variable Node → watch Live Values.

## Core UX

- Main navigation is Address Space tree-first.
- Default layout is two-pane: Address Space tree on the left and a right pane with tabs for Node Details and Watchlist.
- Start browsing at the OPC UA **Objects** root; Types/Views remain accessible as advanced navigation.
- Selecting a Variable Node emphasizes Live Value first: value, health, timestamps/age, data type, and Engineering Unit when available.
- Non-Variable nodes emphasize metadata and relationships.
- Show DisplayName primarily; reveal BrowseName when different.
- Show compact NodeId prominently and make Expanded NodeId copyable.
- Use arrows/Enter/Esc/? as primary controls, with vim-like accelerators.
- `Tab` cycles focus through Address Space, Node Details, and Watchlist; the right pane preserves the last active tab when focus returns to Address Space.
- The Watchlist tab label shows the number of watched Variable Nodes.
- Adding a Variable Node to the Watchlist does not automatically switch tabs.
- Mouse support is optional; keyboard remains complete.
- Persistent footer hints plus `?` help overlay for v1.
- Persistent Read-Only Mode indicator.

## Live values and monitoring

- Use OPC UA subscriptions for selected-node updates.
- Add Watchlist subscriptions after selected-node updates are working.
- Request a 1s sampling interval by default and display the server-revised interval where relevant.
- Mark values as **Stale Values** when connection/subscription state means they may no longer be current.
- Keep OPC UA StatusCode separate from subscription health.
- Warn softly when Watchlist grows large, e.g. after ~50 nodes.
- Support Session Trend for values observed during the current session.
- Show range metadata in details and optionally simple visualizations.
- Treat values outside server-provided ranges as **Out-of-Range**, not alarms.

## Browsing, indexing, and search

- Use lazy browse for tree expansion.
- Use optional background indexing for search.
- Make incomplete knowledge explicit: global indexing status, per-node loaded/loading/failed/unknown state, and search disclaimers.
- Use **Rate-Limited Browsing** with Low/Normal/Fast presets.
- Default indexing scope: Objects root. Allow selected subtree indexing. Full-server indexing must be explicit.
- Search defaults to indexed Objects nodes.
- Direct NodeId lookup should be separate from indexed search.
- Browse failures keep the node visible, mark error state, and allow manual retry.

## Connections and security

- v1 supports Anonymous and username/password authentication.
- Do not store passwords in v1.
- Support common OPC UA security modes/policies as library capability allows.
- For new connections, discover endpoints first and let user choose security/auth settings.
- Saved Connections reuse known-good endpoint/security/auth details.
- Saved Connections store non-secret connection details locally.
- Support OS config directory by default and explicit portable mode.
- For untrusted server certificates, show an interactive trust prompt with certificate details/fingerprint.
- Trust server certificates per Saved Connection in v1, not globally.
- Auto-reconnect visibly with backoff and restore Watchlist/subscriptions when possible.
- Show useful diagnostics: endpoint, security mode/policy, auth type, session state, subscription count, indexing progress, last error.

## Exports and diagnostics

- v1 supports Watchlist **Snapshots**.
- v1 keeps logs in memory during a Troubleshooting Session.
- User can export a **Diagnostics Bundle** containing recent diagnostic logs and session/app diagnostic state.
- Export flow must warn that node names, process values, endpoints, and server metadata may be sensitive.
- Secrets are always redacted.

## Implementation

- Stack: Go + Bubble Tea.
- OPC UA library: start with `github.com/gopcua/opcua`, but validate capability early.
- Architecture: separate OPC UA client service from Bubble Tea model. TUI sends commands; service emits connection/browse/value events.
- Keep an explicit command/action layer so later writes/method calls can be gated without redesigning v1.

## Distribution

- v1 release format: archives containing single binary plus concise docs/example config.
- Target Windows and Linux equally; macOS is nice-to-have for developers.
- CLI entry shapes:
  - `termua`
  - `termua opc.tcp://host:4840`
  - `termua --connection <name>`

## Testing

- Start with pure TUI model tests and OPC UA service integration tests.
- Integration tests use an endpoint from `TERMUA_TEST_ENDPOINT`.
- If no endpoint is configured, integration tests are skipped.
- The mature local `.exe` test server is the preferred development integration target.
- Public remote test server may be used manually or by an optional workflow, but must not block normal CI.
- Unit tests mock a small app-level OPC UA client interface.

## Explicit non-goals for v1

- Writes to Variable Nodes.
- Method calls.
- Alarms & Conditions interactions.
- OPC UA historical reads.
- Persistent dashboards.
- Full credential storage / OS keychain.
- Deep ExtensionObject/schema browser.
- Multi-server sessions.
