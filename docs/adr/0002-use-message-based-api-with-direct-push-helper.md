# Use a message-based API with a direct Push helper

Bubble Toast uses `toast.Show(...)` as the primary API so any Bubble Tea component can request a Toast by returning a command and letting the parent route messages through the Toast model. A direct `Model.Push(...)` helper is also provided for small apps, tests, and callers that already own the Toast model, with both paths sharing the same queueing and dismissal behavior.
