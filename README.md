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
| `template`, `network_mode`, `allowed_hosts`, `env`, `packages`, `agents`, `repo` | Yes |
| `resources.cpu` / `.memory` / `.disk` | Yes — capped at **8 vCPU**, **8192 MB**, **50 GB**. Omit a field (or set `0`) to take the server default. `caged up` checks these locally using the same bounds the API enforces, so a config it accepts is one the API accepts |
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
| `caged mcp servers` | Register and manage third-party MCP servers |
| `caged mcp bind` / `unbind` | Decide which persona sees which server |
| `caged mcp tools` | The pinned tool catalogue, as an agent sees it |
| `caged mcp readiness` | Whether policy will actually allow those tools |
| `caged mcp oauth` | Authorize a third-party server, consent first |
| `caged mcp inputs` | Questions servers have asked, waiting on a person |
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

## Third-party MCP servers

Caged can also let the agent *inside* a sandbox use other people's MCP servers —
GitHub, Linear, Postgres — without widening the sandbox's egress by one byte.
The API dials out; the guest does not. Tools arrive on the connection the agent
already has, namespaced `alias__tool`.

```bash
caged mcp catalogue                                    # servers Caged has reviewed
caged mcp servers add --alias github   --catalogue-id io.github.github/github-mcp-server    # register
caged mcp servers refresh <server-id>                  # fetch and pin its tools
caged mcp bind <server-id> --persona <id> --allow-tools
caged mcp readiness --persona <id>                     # will calls actually work?
```

### Two things that will otherwise cost you an afternoon

**Binding a server does not make its tools callable.** Caged's autonomy tiers
rank *its own* tools (`filesystem_read`, `terminal_exec`, `git_push`). A
third-party name like `github__get_issue` matches none of them, so it is
unclassified — and an unclassified tool is denied at **every** tier, including
`autonomous`. That is deliberate: Caged cannot know whether a stranger's tool
reads an issue or wires money.

`--allow-tools` on `bind` writes the one rule that clears it, `caged mcp allow`
writes it later, and `caged mcp readiness` tells you which servers still need it
rather than leaving you to discover it one refused call at a time.

**A `quarantined` tool is one whose definition CHANGED** since a human approved
it. Caged hashes every tool definition and holds a changed one: a server that is
benign on Monday and poisoned on Tuesday becomes a review rather than a silent
compromise. Read it before deciding:

```bash
caged mcp tools diff <server-id> <tool>     # approved vs. current, parameters named
caged mcp tools approve <server-id> <tool>
caged mcp tools reject  <server-id> <tool> --note "grew a debug_context parameter"
```

### OAuth is two steps on purpose

```bash
caged mcp oauth show <server-id>                       # issuer, scopes, nothing minted
caged mcp oauth consent <server-id> --persona <id>     # your decision; forwards nothing
caged mcp oauth authorize <server-id> --persona <id>   # the URL to open
```

Caged records your consent *before* it forwards anything to a third party's
authorization server. One endpoint that discovered, minted a state and redirected
would be a link that starts a real authorization flow attributed to whoever's
browser followed it.

Caged holds the resulting token itself: sealed at rest, never written into a
sandbox, never in an environment variable, and never returned by any API read.

### When a server asks a question

Under the current MCP revision a server can ask the client a question mid-call.
Caged routes it to a **person**, not to the agent's model — in an unattended run
the alternative is a model answering a stranger's question on your behalf.

```bash
caged mcp inputs
caged mcp inputs respond <id> --answer 'team={"team":"platform"}'
caged mcp inputs respond <id> --decline
```

A *sampling* request — "run an inference on my prompt and give me the
completion" — is refused outright. No approval makes that safe.

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
