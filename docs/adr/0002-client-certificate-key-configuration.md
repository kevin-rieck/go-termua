# ADR 0002: Client certificate and private key configuration

## Status

Accepted

## Context

Some OPC UA Servers advertise secure Server Connection endpoints with `Sign` or `SignAndEncrypt` message security. The OPC UA client library requires client certificate and private key material when connecting to these secure endpoints, even when user authentication remains `Anonymous` or `UserName`.

Certificate-based user authentication is separate and remains future work.

## Decision

TermUA uses a global client application certificate and private key for secure message modes. By default, it creates and reuses a standard self-signed client application certificate under the app config directory:

- `certificates/client-cert.pem`
- `certificates/client-key.pem`

Automation Engineers may override those paths at launch:

- `--client-certificate PATH`
- `--client-private-key PATH`

These paths apply to all secure Server Connections during the current app run. They are not stored in Saved Connection metadata in this slice. Saved Connections may continue to store known-good endpoint, security, and authentication choices without embedding local certificate/key file paths.

The client application certificate is only for OPC UA message security (`Sign` / `SignAndEncrypt`). Certificate-based user authentication remains future work.

## Missing material behavior

When an Automation Engineer selects a secure endpoint (`Sign` or `SignAndEncrypt`) and either the client certificate path or private key path is missing, the Server Connection flow must fail before connecting with a clear, actionable error:

`secure endpoint requires client certificate and private key`

When configured certificate/key files are missing or invalid, the Server Connection flow must fail before connecting with a clear file-specific error.

## Consequences

- First-run usage works without choosing files in the TUI; the generated client application certificate can be trusted in servers that require trust-list approval.
- CLI flags remain available for sites with managed client certificates.
- Certificate user authentication (`UserTokenTypeCertificate`) remains unsupported and is treated as future work.
