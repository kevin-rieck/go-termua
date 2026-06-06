# PRD: Bubble Toast v1

## Problem Statement

Bubble Tea and Lip Gloss app authors need a simple way to show transient, non-blocking Toasts in terminal user interfaces. Without a dedicated component, each app has to reinvent Toast Stack state, Dismissal timing, queueing, placement, rendering, and Bubble Tea message routing. That boilerplate is easy to get wrong, especially when Toasts are updated by Toast ID, expire while queued, or need to be overlaid onto a styled terminal view.

## Solution

Build Bubble Toast as a Go package named `toast`: a Bubble Tea/Lip Gloss-specific component that manages a Toast Stack, queued overflow, Dismissal, Toast IDs, Toast Placement, and rendering. The primary API is message-based so any Bubble Tea component can request a Toast by returning `toast.Show(...)`; a direct `Model.Push(...)` helper is also available for small apps, tests, and callers that already own the Toast model.

The v1 experience should feel batteries-included for common Toasts while preserving escape hatches for Lip Gloss apps that need custom presentation.

## User Stories

1. As a Bubble Tea app author, I want to add a Toast component to my model, so that I can show transient messages without building Toast Stack logic myself.
2. As a Bubble Tea app author, I want to show a Toast from any component via a command, so that nested components do not need direct access to the Toast model.
3. As a Bubble Tea app author, I want to explicitly forward Bubble Tea messages to the Toast model, so that routing stays idiomatic and testable.
4. As a small app author, I want to push a Toast directly into the Toast model, so that simple apps can avoid message indirection.
5. As a library user, I want constructors for info, success, warning, and error Toast Kinds, so that common Toasts are easy to create.
6. As a library user, I want a neutral Toast constructor, so that I can show a Toast without assigning a semantic Toast Kind.
7. As a library user, I want Toast Kind defaults to provide sensible styling, so that Toasts look useful without configuration.
8. As a library user, I want Toast Kinds to be optional, so that Bubble Toast does not force every Toast into a semantic category.
9. As a library user, I want to set a Toast title separately from its message, so that I can render richer Toast Content.
10. As a library user, I want to provide pre-rendered Toast Content, so that I can fully control the body with Lip Gloss when needed.
11. As a library user, I want pre-rendered Toast Content to replace title/message rendering, so that rendering precedence is predictable.
12. As a library user, I want each Toast to auto-dismiss by default, so that transient messages do not require manual cleanup.
13. As a library user, I want to set a model-level default duration, so that most Toasts share a consistent lifetime.
14. As a library user, I want to override duration per Toast, so that important Toasts can remain visible longer.
15. As a library user, I want to mark a Toast as persistent, so that important short-lived status can remain until explicit Dismissal.
16. As a library user, I want to dismiss a specific Toast by Toast ID, so that long-running status can be removed precisely.
17. As a library user, I want to dismiss all Toasts, so that the app can clear both visible and queued Toasts at once.
18. As a library user, I want generated Toast IDs for anonymous Toasts, so that I do not need to provide IDs for ordinary messages.
19. As a library user, I want caller-provided Toast IDs, so that I can update or dismiss long-running Toasts.
20. As a library user, I want showing a Toast with an existing Toast ID to replace the existing Toast, so that there is at most one Toast for a stable identity.
21. As a library user, I want replacing a Toast to restart its timer, so that updated Toast Content receives a full display duration.
22. As a library user, I want stale expiration timers to be ignored after replacement, so that an old timer cannot dismiss a newer Toast with the same Toast ID.
23. As a library user, I want multiple Toasts visible at once, so that bursts of app events can be communicated.
24. As a library user, I want a configurable maximum visible Toast Stack size, so that Toasts do not overwhelm the terminal UI.
25. As a library user, I want overflow Toasts to queue, so that Toasts are not immediately lost when the visible Toast Stack is full.
26. As a library user, I want the queue to be bounded by default, so that stale Toasts and memory growth are avoided during event bursts.
27. As a library user, I want oldest queued Toasts dropped when the queue is full, so that newer feedback remains relevant.
28. As a library user, I want matching Toast ID updates to bypass queue capacity limits, so that stable status updates are not dropped.
29. As a library user, I want queued Toast timers to start only when they become visible, so that every visible Toast receives its intended duration.
30. As a library user, I want queued Toasts to fill visible slots immediately after Dismissal, so that the queue drains predictably.
31. As a library user, I want persistent Toasts to count against the visible Toast Stack limit, so that layout constraints remain honest.
32. As a Bubble Tea app author, I want preset Toast Placements, so that I can place the Toast Stack without custom layout math.
33. As a Bubble Tea app author, I want top-left, top-right, top-center, bottom-left, bottom-right, and bottom-center Toast Placements, so that common terminal layouts are covered.
34. As a Bubble Tea app author, I want top-right as the default Toast Placement, so that the component has a conventional default.
35. As a Bubble Tea app author, I want newest Toasts closest to the configured screen edge, so that recent events are most prominent.
36. As a Bubble Tea app author, I want a composable Toast Stack view, so that I can integrate Bubble Toast into custom layouts.
37. As a Bubble Tea app author, I want an overlay helper, so that I can place the Toast Stack over an existing rendered view quickly.
38. As a Bubble Tea app author, I want overlay helpers to use Lip Gloss-aware measurement, so that ANSI styling and wide characters are handled correctly.
39. As a Bubble Tea app author, I want overlay helpers to use stored window size when available, so that full-screen placement works after resize messages.
40. As a Bubble Tea app author, I want overlay helpers to accept explicit dimensions, so that I can place Toasts inside subregions and tests.
41. As a Bubble Tea app author, I want configurable overlay margins, so that Toasts do not have to touch the terminal edge.
42. As a Bubble Tea app author, I want margins to affect only overlay placement, so that the composable Toast Stack view remains clean.
43. As a terminal UI designer, I want Toast width to be configurable, so that Toasts fit my layout.
44. As a terminal UI designer, I want Toast Content to wrap to the configured width, so that long messages do not break placement.
45. As a terminal UI designer, I want a configurable maximum Toast height, so that a long Toast cannot dominate the screen.
46. As a terminal UI designer, I want overflowing Toast Content truncated with an ellipsis, so that the Toast Stack remains bounded.
47. As a terminal UI designer, I want configurable gap between Toasts, so that the Toast Stack can be dense or spacious.
48. As a terminal UI designer, I want default styles by Toast Kind, so that common Toasts have a useful appearance immediately.
49. As a terminal UI designer, I want to override styles by Toast Kind, so that Toasts match my app theme.
50. As a terminal UI designer, I want to replace the whole theme, so that I can configure all built-in Toast Kinds consistently.
51. As a terminal UI designer, I want a custom renderer escape hatch, so that I can define full Toast presentation when styles are not enough.
52. As a custom renderer author, I want render context with width, max height, resolved style, Toast Placement, index, and total, so that rendering can adapt to layout.
53. As a library user, I want no icons by default, so that the component remains terminal/font compatible.
54. As a library user, I want no built-in animation in v1, so that the component remains predictable and simple.
55. As a library user, I want no global Toast bus, so that state stays local to the Bubble Tea model.
56. As a library user, I want no lifecycle callbacks in v1, so that the API remains idiomatic and small.
57. As a library user, I want no priority system in v1, so that Toast ordering remains predictable.
58. As a library user, I want unknown Toast Kinds to render like the neutral kind, so that rendering is robust.
59. As a library user, I want invalid model option values normalized safely, so that construction remains simple and non-panicking.
60. As a maintainer, I want deterministic tests for queueing, replacement, Dismissal, and rendering behavior, so that subtle Toast Stack bugs do not regress.
61. As a maintainer, I want a minimal Bubble Tea example, so that public API ergonomics are validated in a real app.

## Implementation Decisions

- Build a Go module whose root package is named `toast`.
- The public package depends directly on Bubble Tea and Lip Gloss, in line with the accepted architectural decision to target those libraries directly.
- Use a value-type `Model` with `Init() tea.Cmd`, `Update(msg tea.Msg) (Model, tea.Cmd)`, `View() string`, direct helper methods, and overlay helpers.
- Use functional options for model construction: defaults are initialized first, then options are applied.
- Provide separate model options and Toast options to avoid ambiguous option names.
- Model options include default duration, maximum visible Toast Stack size, maximum queued Toasts, Toast Placement, width, max height, gap, overlay margin, theme, style override, and renderer override.
- Toast options include duration, persistent behavior, Toast ID, title, and Toast Kind.
- Use exported semantic Toast fields and keep runtime generation/timer state internal.
- Use `type ID string`; empty Toast ID means the model generates an ID.
- Generated IDs are owned by each model, avoiding package-level mutable state.
- Direct push returns the updated model, assigned Toast ID, and a command for timer scheduling.
- The message-based command API is primary: showing, dismissing one Toast, and dismissing all Toasts are exposed as command constructors.
- Export Bubble Tea message types for showing and Dismissal so tests and advanced apps can route messages directly.
- Keep timer expiration messages internal.
- Expiration messages include Toast ID plus a generation token so stale timers do not dismiss replacement Toasts.
- Showing a Toast with an existing Toast ID replaces the matching Toast wherever it is, visible or queued.
- Replacement preserves visible/queued position and restarts the timer only if the Toast is visible.
- Queue timers start only when a Toast becomes visible.
- When visible Toasts are dismissed or expire, queued Toasts fill visible capacity immediately and their timers are scheduled.
- Maximum visible Toast Stack defaults to 3. Values less than or equal to zero normalize to the default.
- Maximum queued Toasts defaults to 20. An explicit zero disables queueing; negative values normalize to the default.
- If the queue is full, the oldest queued Toast is dropped silently. Toasts are best-effort UI feedback, not an audit log.
- Queue limits apply only to new Toast identities; matching Toast ID updates always win.
- Persistent Toasts count against the maximum visible Toast Stack size.
- Dismissal by Toast ID removes the Toast from both the visible Toast Stack and the queue.
- Dismissal of all Toasts removes visible and queued Toasts.
- Direct Dismissal methods return an updated model and command for consistency, even when the command is nil.
- Returned commands are batched internally when multiple commands must be scheduled.
- Toast Kinds are `KindNone`, `KindInfo`, `KindSuccess`, `KindWarning`, and `KindError`. Unknown values render like `KindNone`.
- Convenience constructors exist for neutral, info, success, warning, and error Toasts. Constructors take a message and variadic Toast options.
- `New` constructs the model. `NewToast` constructs a neutral Toast.
- `WithID` and public Dismissal helpers accept strings for call-site ergonomics and convert internally to `ID`.
- `Content` replaces title/message rendering when present. Default rendering precedence is custom model renderer, then Toast Content, then title plus message.
- `View()` renders only the Toast Stack, not overlay padding or full-screen placement.
- Toast Placement affects Toast Stack render order: top placements render newest at top, bottom placements render newest at bottom.
- Include top-left, top-right, top-center, bottom-left, bottom-right, and bottom-center placements. Do not include center-screen placement or placement aliases in v1.
- Overlay helpers place the Toast Stack over a base view using stored dimensions when available, or measured base dimensions as fallback. If base is empty or zero-sized, they return the Toast Stack view.
- Overlay helpers are best-effort in v1 and do not perform complex clipping beyond Toast width wrapping and max-height truncation.
- Default width is 40 and means total rendered width, including border and padding.
- Default max height is 4 and means total rendered height, including border and padding. An explicit max height of zero means unlimited; negative values normalize to default.
- Default gap between Toasts is 1. An explicit gap of zero is valid; negative values normalize to default.
- Default overlay margin is 1 on every side. Margin affects overlay helpers only, not `View()`.
- Theme uses struct fields for built-in Toast Kinds, not a map. No custom Toast Kind support is promised in v1.
- No icons are included in the v1 default theme. Users can add icons through custom content or a renderer.
- No animation, pause/resume, lifecycle hooks, priority system, built-in input handling, or global bus are included in v1.
- Minimum Go version is Go 1.22 unless Bubble Tea or Lip Gloss require a higher version.

## Testing Decisions

- Test at the highest available seam: the public model API and Bubble Tea message API. Direct internals should be exercised only where required for deterministic timer behavior.
- Core behavior tests should initially live in the same package as the component so internal expiration messages and generation tokens can be tested without sleeping.
- External-package tests or examples can be added for public API ergonomics after core behavior is stable.
- Avoid real-time sleeps in tests. Expiration behavior should be tested by sending controlled internal expiration messages.
- Test showing a Toast through the direct `Push` path and through the message-based `Show` path.
- Test generated Toast IDs and caller-provided Toast IDs.
- Test that replacement by Toast ID updates visible Toasts and queued Toasts without creating duplicates.
- Test that replacement restarts visible timers by advancing generation tokens.
- Test that stale expiration messages are ignored after replacement.
- Test that queued Toasts do not start timers until they become visible.
- Test visible Toast Stack capacity, default maximum visible count, and immediate queue draining after Dismissal.
- Test bounded queue behavior, including explicit queue disabling and dropping the oldest queued Toast when full.
- Test that matching Toast ID updates bypass queue capacity limits.
- Test that persistent Toasts remain until explicit Dismissal and count against visible capacity.
- Test Dismissal by Toast ID across both visible and queued Toasts.
- Test Dismissal of all Toasts across both visible and queued Toasts.
- Test render order for top and bottom Toast Placements.
- Test `Visible()` returns a copy in render order, `Queued()` returns a copy in FIFO order, and `Len()` counts visible plus queued.
- Test default rendering precedence: renderer override, Toast Content, then title/message.
- Test width wrapping, max-height truncation, unlimited max height, gap handling, and invalid option normalization.
- Test overlay helpers with explicit dimensions and with fallback measured base dimensions.
- Use the basic example as an integration smoke test where practical.

## Out of Scope

- Framework-agnostic core package or adapters for non-Bubble Tea frameworks.
- Package-level global Toast bus or global state.
- Center-screen/modal placement.
- Animation.
- Pause-on-hover, pause-on-focus, or pause/resume timer controls.
- Built-in keyboard or mouse handling for Dismissal.
- Lifecycle callbacks or lifecycle messages such as shown/dismissed/dropped events.
- Priority or kind-based queue reordering.
- Guaranteed Toast delivery or audit-log semantics.
- Custom Toast Kinds as a supported public feature.
- Default icons or icon theming.
- Complex overlay clipping beyond wrapping and max-height truncation.
- Multiple example apps beyond the initial basic example.

## Further Notes

Accepted architectural decisions already exist for targeting Bubble Tea/Lip Gloss directly and for using the message-based API with a direct Push helper. The initial implementation should start with deterministic model behavior tests, then add rendering and overlay behavior, then add a minimal Bubble Tea example using the message-based API.
