# Caged CLI

The official command-line interface for [Caged](https://caged.dev) — the AI Agent Sandbox Platform.

## Install

```bash
# Homebrew (macOS/Linux)
brew tap caged-dev/tap
brew install caged

# Or download from GitHub Releases
# https://github.com/caged-dev/cli/releases
```

## Quick Start

```bash
# Authenticate with your API key
caged login

# Create a sandbox
caged sandboxes create --template node --cpus 2 --memory 1024

# List your sandboxes
caged list

# Connect to a sandbox
caged connect cage_abc123

# Execute a command
caged exec cage_abc123 "npm test"

# Pause (stops billing)
caged sleep cage_abc123

# Resume
caged wake cage_abc123

# Destroy
caged destroy cage_abc123
```

## Config-as-Code

Create a `.caged.yaml` in your repo root:

```yaml
template: node    # Alias for node-22 (also: python → python-312)
resources:
  cpu: 2
  memory: 1024
  disk: 10
network_mode: allowlist
allowed_hosts:
  - registry.npmjs.org
  - github.com
timeout: 900      # Idle seconds before the sandbox sleeps
budget: 5.00      # Recorded and reported against — not a cap
init_script: "npm install"
secrets:
  - ANTHROPIC_API_KEY
env:
  NODE_ENV: development
```

Then just run:
```bash
caged up
```

### What each key does

| Key | Applied by the API? |
|---|---|
| `template`, `resources`, `network_mode`, `allowed_hosts`, `env`, `packages`, `agents`, `repo` | Yes |
| `timeout` | Yes — clamped to your plan's maximum idle timeout |
| `budget` | Recorded, and `caged list` reports cost against it. **Nothing enforces it**: a sandbox that passes its budget keeps running |
| `secrets`, `init_script` | Sent, but **no current API path applies them**. `caged up` prints a warning when you set either; run the equivalent with `caged exec` in the meantime |

Flags override the file: `caged up --timeout 1800 --cpus 4`.

## Commands

| Command | Description |
|---------|-------------|
| `caged login` | Configure API credentials |
| `caged run` | Create and start a sandbox |
| `caged up` | Create sandbox from `.caged.yaml` |
| `caged list` | List sandboxes |
| `caged connect <id>` | Interactive terminal session |
| `caged exec <id> <cmd>` | Execute a command |
| `caged destroy <id>` | Destroy a sandbox |
| `caged sleep <id>` | Pause sandbox (saves costs) |
| `caged wake <id>` | Resume a sleeping sandbox |
| `caged logs <id>` | Show sandbox events (`-f` to follow, `--tail N` for window size) |
| `caged mcp <id>` | Run an MCP server over stdio, bridged to a sandbox |
| `caged version` | Show version |

## MCP (Claude Desktop / Cursor)

`caged mcp` speaks the Model Context Protocol over stdin/stdout and proxies
tool calls (filesystem, terminal, git) into a running sandbox — no WebSocket
setup required. Add it to your MCP client config:

```json
{
  "mcpServers": {
    "caged": {
      "command": "caged",
      "args": ["mcp", "cage_abc123"]
    }
  }
}
```

The sandbox must be running (`caged wake <id>` if it's sleeping).

## Configuration

Config is stored at `~/.config/caged/config.json`:

```json
{
  "api_url": "https://api.caged.dev",
  "api_key": "caged_..."
}
```

Override the API URL for self-hosted deployments:
```bash
caged login  # enter custom URL when prompted
```

## License

MIT — see [LICENSE](LICENSE)
