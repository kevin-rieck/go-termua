# Bubble Toast

Bubble Toast provides transient, non-blocking messages for terminal user interfaces built with Bubble Tea and Lip Gloss.

## Language

**Toast**:
A transient, non-blocking message shown in a terminal UI to acknowledge an event or communicate short-lived status.
_Avoid_: Notification, alert, flash message

**Toast Stack**:
The ordered set of Toasts currently visible at the same time. Bubble Toast may limit the Toast Stack to avoid overwhelming the terminal UI.
_Avoid_: Notification list, inbox

**Toast Kind**:
An optional category that communicates the intent of a Toast, such as info, success, warning, or error. Toast Kinds provide sensible default presentation without preventing custom rendering.
_Avoid_: Severity, level, type

**Dismissal**:
The removal of a Toast from Bubble Toast, whether the Toast is currently visible or waiting to be shown.
_Avoid_: Close, clear, delete

**Toast ID**:
A stable identity for a Toast, used when the host app needs to update or dismiss a specific Toast. Bubble Toast can create Toast IDs automatically, while host apps may provide one for long-running or replaceable messages.
_Avoid_: Key, handle

**Toast Placement**:
The area of the terminal UI where the Toast Stack is presented, such as top-right or bottom-center. Toast Placement describes presentation intent without requiring a specific layout implementation.
_Avoid_: Position, anchor

**Toast Content**:
The user-facing text or rendered body of a Toast, usually expressed as an optional title and a message. Toast Content may be pre-rendered when the host app needs full control over presentation.
_Avoid_: Payload, body
