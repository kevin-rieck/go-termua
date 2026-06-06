# ADR 0002: Client certificate and private key configuration

## Status

Accepted

## Context

Some OPC UA Servers advertise secure Server Connection endpoints with `Sign` or `SignAndEncrypt` message security. The OPC UA client library requires client certificate and private key material when connecting to these secure endpoints, even when user authentication remains `Anonymous` or `UserName`.

Certificate-based user authentication is separate and remains future work.

## Decision

TermUA receives client certificate and private key paths as global launch configuration:

- `--client-certificate PATH`
- `--client-private-key PATH`

These paths apply to all secure Server Connections during the current app run. They are not stored in Saved Connection metadata in this slice. Saved Connections may continue to store known-good endpoint, security, and authentication choices without embedding local certificate/key file paths.

Future config-file support may persist the same global values, but CLI flags are the first supported source.

## Missing material behavior

When an Automation Engineer selects a secure endpoint (`Sign` or `SignAndEncrypt`) and either the client certificate path or private key path is missing, the Server Connection flow must fail before connecting with a clear, actionable error:

`secure endpoint requires client certificate and private key`

## Consequences

- First-run usage is explicit: pass both CLI flags when secure endpoints are needed.
- Repeat usage can be supported later by global config without changing `ConnectRequest` semantics.
- Certificate user authentication (`UserTokenTypeCertificate`) remains unsupported and is treated as future work.
