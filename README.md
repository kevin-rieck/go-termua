# TermUA 🌐

TermUA is a blazing fast, terminal-based user interface (TUI) for exploring and interacting with OPC-UA servers. Built with Go and Bubble Tea, it brings industrial automation data right to your command line without the overhead of heavy graphical clients.

## Features

- **Quick Connection**: Discover endpoints automatically and securely authenticate with your OPC-UA servers.
- **Address Space Explorer**: Intuitive tree view to browse objects and variable nodes effortlessly.
- **Live Watchlist**: Subscribe to real-time value changes for the variables you care about.
- **Deep Node Inspection**: Instantly view metadata, access levels, and data types for any node.
- **Zero Configuration**: Single portable binary. No Java, no heavy desktop frameworks.

## Installation

Download the latest pre-compiled binary for your operating system and architecture from the [Releases page](https://github.com/kevin-rieck/go-termua/releases).

We support Windows and Linux (amd64 & arm64).

```bash
# Example for Linux amd64
wget https://github.com/kevin-rieck/go-termua/releases/download/v0.1.0/termua-linux-amd64
chmod +x termua-linux-amd64
./termua-linux-amd64
```

## Usage Guide

When you launch TermUA, you'll be greeted with the main layout consisting of the Address Space tree on the left and the Inspection/Watchlist panes on the right.

![Startup](assets/startup.gif)

### Connecting to a Server

Press `c` to open the connection wizard. Enter the OPC-UA server address (e.g., `opc.tcp://localhost:4840`).

TermUA will automatically query the server to discover available endpoints and security policies:

![Endpoint Discovery](assets/endpoint-discovery.gif)

Select your preferred endpoint and authentication method (Anonymous or Username/Password).

![Connection Modal](assets/connection-modal.gif)

*Connection errors are handled gracefully with non-blocking toast notifications.*

![Connection Error](assets/connection-error-toast.gif)

### Browsing and Inspecting Nodes

Navigate the Address Space tree using your arrow keys or vim-like bindings (`j`/`k`). 
Press `Enter` or `l` to expand objects and browse their children.

As you navigate, the right pane will automatically display detailed metadata about the selected variable node, including its Access Level, Data Type, and Description.

### The Watchlist & Subscriptions

To monitor a variable's value in real-time, highlight it in the tree and press `w` to add it to your Watchlist. 
TermUA will subscribe to the node and stream live updates directly to your terminal.

![Watchlist Subscription](assets/watchlist-subscription.gif)

Use `Tab` to switch focus between the Address Space tree, the Inspection pane, and your Watchlist.

![Watchlist Tab](assets/watchlist-tab.gif)

## Keybindings

| Key | Action |
| --- | --- |
| `c` | Open Connection Modal |
| `Tab` / `Shift+Tab` | Cycle focus between panes |
| `Up/Down` or `k/j` | Navigate lists and tree nodes |
| `Right/Enter` or `l` | Expand tree node / Select item |
| `Left` or `h` | Collapse tree node |
| `w` | Add selected node to Watchlist |
| `s` | Export snapshot of current data |
| `?` | Toggle help menu |
| `q` or `Ctrl+C` | Quit TermUA cleanly |

## License

MIT License.
