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
budget: 5.00
init_script: "npm install"
env:
  NODE_ENV: development
```

Then just run:
```bash
caged up
```

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
| `caged logs <id>` | Stream sandbox events |
| `caged version` | Show version |

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
