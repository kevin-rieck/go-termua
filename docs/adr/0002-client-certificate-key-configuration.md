# ADR 0002: Client certificate and private key configuration

## Status

Accepted

## Context

Some OPC UA Servers advertise secure Server Connection endpoints with `Sign` or `SignAndEncrypt` message security. The OPC UA client library requires client certificate and private key material when connecting to these secure endpoints, even when user authentication remains `Anonymous` or `UserName`.

Some OPC UA Servers also advertise certificate-based user authentication (`UserTokenTypeCertificate`). TermUA uses the same configured/generated client certificate and private key as the certificate user credential in this slice.

## Decision

TermUA uses a global client application certificate and private key for secure message modes. By default, it creates and reuses a standard self-signed client application certificate under the app config directory:

- `certificates/client-cert.pem`
- `certificates/client-key.pem`

Automation Engineers may override those paths at launch:

- `--client-certificate PATH`
- `--client-private-key PATH`

These paths apply to all secure Server Connections and certificate-authenticated Server Connections during the current app run. They are not stored in Saved Connection metadata in this slice. Saved Connections may continue to store known-good endpoint, security, and authentication choices without embedding local certificate/key file paths.

The client application certificate is used for OPC UA message security (`Sign` / `SignAndEncrypt`) and, when selected, certificate-based user authentication. A separate user credential certificate/key remains future work if real servers require it.

## Missing material behavior

When an Automation Engineer selects a secure endpoint (`Sign` or `SignAndEncrypt`) or `Certificate` user authentication and either the client certificate path or private key path is missing, the Server Connection flow must fail before connecting with a clear, actionable error:

`certificate connection requires client certificate and private key`

When configured certificate/key files are missing or invalid, the Server Connection flow must fail before connecting with a clear file-specific error.

## Consequences

- First-run usage works without choosing files in the TUI; the generated client application certificate can be trusted in servers that require trust-list approval.
- CLI flags remain available for sites with managed client certificates.
- Certificate user authentication (`UserTokenTypeCertificate`) is supported by reusing the global TermUA client certificate/key. Separate user-certificate configuration remains future work.
