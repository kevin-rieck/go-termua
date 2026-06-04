# OPC UA Client TUI

A terminal user interface for automation engineers who need to inspect and interact with existing OPC UA Servers.

## Language

**OPC UA Client TUI**:
A terminal application that connects to existing OPC UA Servers so automation engineers can inspect and interact with them.
_Avoid_: OPC UA Server TUI, terminal UI for servers

**Read-Only Mode**:
A state of the OPC UA Client TUI where it does not perform write operations or method calls against the OPC UA Server.
_Avoid_: Safe mode, view-only mode, no-write mode

**OPC UA Server**:
An industrial automation endpoint that exposes data, metadata, events, and operations through the OPC UA protocol.
_Avoid_: Server implementation, backend

**Automation Engineer**:
A practitioner who configures, commissions, troubleshoots, or maintains industrial automation systems.
_Avoid_: Developer, operator, user

**Troubleshooting Session**:
A focused investigation of a live OPC UA Server to understand current values, metadata, status, and connectivity problems.
_Avoid_: Monitoring session, dashboard, commissioning workflow

**Address Space**:
The browsable structure of an OPC UA Server, containing nodes and their relationships.
_Avoid_: Tag tree, menu, file tree

**Rate-Limited Browsing**:
Browsing or indexing the Address Space at a bounded request rate to reduce load on an OPC UA Server.
_Avoid_: Gentle browsing, crawl, aggressive indexing

**Variable Node**:
A node in the Address Space that represents a readable, and sometimes writable, process value or state.
_Avoid_: Tag, point, field

**Live Value**:
The current value of a Variable Node together with its health and timestamp information.
_Avoid_: Reading, datapoint, sample

**Variable Node Inspection**:
The focused view of one Variable Node during a Troubleshooting Session, combining its Live Value, metadata, health, Stale Value state, and Out-of-Range status.
_Avoid_: selected value panel, node details workflow

**Engineering Unit**:
The physical unit associated with a Variable Node's Live Value, when exposed by the OPC UA Server.
_Avoid_: Unit, label, suffix

**Out-of-Range**:
A condition where a numeric Live Value falls outside range metadata exposed by the OPC UA Server.
_Avoid_: Alarm, alert, warning

**Stale Value**:
A previously observed Live Value that may no longer represent the current state of the OPC UA Server.
_Avoid_: Cached value, invalid value, old reading

**Watchlist**:
A user-selected set of Variable Nodes whose Live Values remain visible during a Troubleshooting Session.
_Avoid_: Dashboard, monitor, pinned nodes

**Session Trend**:
A temporary view of how a Live Value changes during the current Troubleshooting Session.
_Avoid_: Historian, chart, trend history

**Snapshot**:
A point-in-time capture of selected troubleshooting information, such as Watchlist Live Values and node details.
_Avoid_: Report, export, dump

**Diagnostics Bundle**:
A user-triggered export of information useful for troubleshooting the OPC UA Client TUI itself, such as connection state, recent errors, and internal diagnostic logs.
_Avoid_: Diagnostic logs, log export, support bundle

**Server Connection**:
An active relationship between the OPC UA Client TUI and an OPC UA Server, including endpoint, security, and authentication choices.
_Avoid_: Session, profile, login

**Saved Connection**:
A locally stored set of non-secret details used to reconnect to an OPC UA Server.
_Avoid_: Profile, bookmark, credential
