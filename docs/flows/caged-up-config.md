# Flow: `.caged.yaml` to a created sandbox

Every key the CLI parses has to end up on the wire. A key parsed into
`cagefile.Config` and absent from `api.CreateSandboxRequest` is a setting the
user wrote, the CLI accepted, and nobody applied — which is what happened to
`timeout`, `secrets` and `init_script`.

```mermaid
flowchart TD
    Y[".caged.yaml"] -->|cagefile.Parse| C["cagefile.Config<br/>env ${VAR} expanded<br/>resource aliases merged"]
    F["CLI flags<br/>--template --cpus --memory --disk<br/>--network --allowlist --timeout<br/>--budget --env --packages --agents<br/>--repo*"] -->|"Config.Merge (flags win)"| C
    C -->|Config.Validate| V{"valid?"}
    V -->|no| E["exit: config error<br/>(bounds mirrored from the API,<br/>so a local pass is a server pass)"]
    V -->|yes| B["buildCreateRequest<br/>every Config field mapped"]
    B --> W["POST /v1/sandboxes<br/>api.CreateSandboxRequest"]
    B --> N["warnUnappliedConfig → stderr<br/>secrets, init_script"]
    W --> S["api.SandboxResponse"]
    S --> P["printSandboxInfo<br/>status, template, ip,<br/>budget (not enforced), cost"]
```

## Resource ceilings

`Config.Validate` checks resources against `cagefile.DefaultLimits()` — 8
vCPU, 8192 MB, 50 GB — which mirror `(*CreateSandboxRequest).Validate` in
`caged-api/internal/api/request.go`. **The API's bounds are the real ones**;
the CLI only restates them so the failure happens before the round trip.

They were once the CLI's own invention (16 / 32768 / 100), and everything in
between passed `caged up` and was then refused by `POST /v1/sandboxes` —
the user was told their config was fine and contradicted one call later.
`internal/cagefile/limits.go` is now the single place the numbers live, and
it names the API file it has to track. The durable fix is server-side: a
limits endpoint the CLI reads, with these constants as the offline fallback.
`Config.ValidateWithLimits` already takes the ceilings as a parameter so
that lands as one new call site.

## Field map

| `.caged.yaml` | request JSON | server behaviour |
|---|---|---|
| `template` | `template` | validated, aliases resolved |
| `resources.cpu` / `.memory` / `.disk` | `cpus` / `memory_mb` / `disk_gb` | bounded at **8 vCPU / 8192 MB / 50 GB**; `0` means "server default" |
| `network_mode`, `allowed_hosts` | `network_mode`, `allowlist` | allowlist required when mode is `allowlist` |
| `env` | `env` | key must not contain `=`, space, tab or newline; merged with any Environment the Persona/Harness carries |
| `packages`, `agents` | `packages`, `agents` | installed at boot |
| `repo.*` | `repo`, `repo_token`, `repo_branch`, `repo_commit`, `repo_subdir` | cloned at boot; `repo.token_env` is read locally, never sent as a name |
| `timeout` | `timeout` | idle timeout, clamped to the plan maximum, negative rejected |
| `budget` | `budget` | recorded only — no enforcement anywhere in the platform |
| `secrets` | `secrets` | decoded by the API, not applied by any current path |
| `init_script` | `init_script` | decoded by the API, not applied by any current path |

## Response fields

`api.Sandbox` declares only fields the API populates. `cost` is always sent,
including zero, so `$0.00` is a real figure. `init_script`, `timeout` and
`config` appear in the API's response type but no server path fills them in,
so the client does not decode them — a permanently-zero field is
indistinguishable from data, which is how a `spent` field that no API has
ever sent reported `$0.00` spend for every sandbox.
