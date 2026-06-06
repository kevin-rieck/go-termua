# Bubble Toast

Bubble Toast is a Bubble Tea/Lip Gloss component for transient, non-blocking Toasts in terminal UIs.

## Run the example

```sh
go run ./examples/basic
```

Press `t` to show Toasts and `q` to quit.

## Basic usage

```go
type model struct {
    toasts toast.Model
}

func (m model) Init() tea.Cmd {
    return toast.Show(toast.Info("ready", toast.WithTitle("App")))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    m.toasts, cmd = m.toasts.Update(msg)
    return m, cmd
}

func (m model) View() string {
    return m.toasts.Overlay("your app view")
}
```

Small apps can also push directly:

```go
m := toast.New()
m, id, cmd := m.Push(toast.Success("saved"))
_, _ = id, cmd
```

## Visual verification

VHS tapes run in Docker and write generated files under ignored `artifacts/`:

```sh
make -f Makefile.tapes capture-tapes
```
