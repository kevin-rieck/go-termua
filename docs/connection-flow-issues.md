# Server Connection Flow Issues

## Context

Review of the Server Connection flow found that the OPC UA Client TUI discovers all advertised endpoints, but the backend only fully supports a subset of the advertised security and authentication combinations.

Supported today:

- Message security: `None` with `SecurityPolicy=None`
- Authentication: `Anonymous`
- Authentication: `UserName` in the backend, with an incomplete username-only endpoint flow

Not supported today:

- Message security: `Sign`
- Message security: `SignAndEncrypt`
- Authentication: `Certificate`
- Authentication: `IssuedToken`

Certificate authentication is explicitly future work. The near-term goal is to avoid misleading fallback behavior and make username/password plus secure-channel support clear and testable.

---

## 1. Support username-only Server Connection endpoints

**Type**: AFK  
**Blocked by**: None - can start immediately

### What to build

When a discovered endpoint advertises only `UserName` authentication, the Server Connection flow should collect credentials and connect with the supplied username/password. The flow should not stop at an endpoint-requires-credentials dead end.

### Acceptance criteria

- [x] A username-only endpoint transitions into credential entry from the endpoint selection flow.
- [x] Submitted credentials produce a `ConnectRequest` using `UserName` auth.
- [x] Existing anonymous and multi-auth endpoint behavior continues to work.
- [x] Tests cover the username-only endpoint path end-to-end through connection state and request generation.

---

## 2. Reject unsupported authentication modes explicitly

**Type**: AFK  
**Blocked by**: None - can start immediately

### What to build

If an endpoint advertises authentication modes that are not currently supported, the connection flow should fail early with clear feedback instead of silently attempting anonymous authentication. `Certificate` authentication should be treated as future work, not implemented in this slice.

### Acceptance criteria

- [x] Selecting `Certificate` auth does not produce an anonymous `ConnectRequest`.
- [x] Selecting `IssuedToken` auth does not produce an anonymous `ConnectRequest`.
- [x] Unsupported auth produces a clear status/error message in the Server Connection flow.
- [x] Backend connection code rejects unknown `AuthType` values explicitly.
- [x] Tests cover unsupported auth in both connection state and backend request handling where practical.

---

## 3. Decide client certificate/key configuration for secure endpoints

**Type**: HITL  
**Blocked by**: None - can start immediately

### What to build

Decide how the OPC UA Client TUI should receive client certificate and private key material for secure Server Connections. This decision should cover first-run behavior, repeat usage, and whether certificate/key paths belong in CLI flags, config files, saved connection metadata, or an interactive prompt.

### Acceptance criteria

- [x] The chosen source of client certificate/key material is documented.
- [x] The decision states whether cert/key paths are global app configuration or per Saved Connection metadata.
- [x] The decision states how missing certificate/key material should be reported when selecting secure endpoints.
- [x] Certificate authentication remains out of scope and is recorded as future work.

---

## 4. Connect to `Sign` endpoints using configured client certificate/key

**Type**: AFK  
**Blocked by**: Issue 3

### What to build

A discovered endpoint with a non-`None` security policy and `Sign` message security mode should connect when client certificate/key material is configured. If certificate/key material is missing, the flow should report a clear error before or during connection.

### Acceptance criteria

- [x] `ConnectRequest` can carry or resolve configured client certificate/key material according to the approved configuration decision.
- [x] Backend connection setup passes certificate and private key options to the OPC UA client for secure policies.
- [x] `Sign` endpoints with configured cert/key no longer fail because of missing private key configuration.
- [x] Missing cert/key for a `Sign` endpoint produces a clear, actionable error.
- [x] Tests cover request construction and missing-cert/key failure behavior.

---

## 5. Connect to `SignAndEncrypt` endpoints using configured client certificate/key

**Type**: AFK  
**Blocked by**: Issues 3 and 4

### What to build

A discovered endpoint with a non-`None` security policy and `SignAndEncrypt` message security mode should connect when client certificate/key material is configured. Missing certificate/key material should produce clear feedback.

### Acceptance criteria

- [x] Backend connection setup supports `SignAndEncrypt` using the configured certificate/key material.
- [x] `SignAndEncrypt` endpoints with configured cert/key no longer fail because of missing private key configuration.
- [x] Missing cert/key for a `SignAndEncrypt` endpoint produces a clear, actionable error.
- [x] Tests cover request construction and missing-cert/key failure behavior.

---

## 6. Show connection capability status in endpoint discovery

**Type**: AFK  
**Blocked by**: Issues 2 and 3

### What to build

The endpoint discovery view should tell the Automation Engineer whether each advertised endpoint is currently connectable, needs username/password, needs client certificate/key configuration, or uses unsupported authentication. This should reduce trial-and-error in the Server Connection flow.

### Acceptance criteria

- [x] Endpoint rows indicate when username/password is required.
- [x] Endpoint rows indicate when client certificate/key configuration is required for secure message modes.
- [x] Endpoint rows indicate unsupported authentication modes such as `Certificate` and `IssuedToken`.
- [x] The default endpoint selection prefers currently connectable endpoints where possible.
- [x] Tests cover capability labels and default selection behavior.

---

## Future work

- Certificate-based user authentication (`UserTokenTypeCertificate`).
- Issued-token authentication (`UserTokenTypeIssuedToken`).
- Trust-store management and server certificate trust decisions, if needed beyond the OPC UA client library defaults.
