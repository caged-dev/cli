package cagefile

// Resource ceilings enforced by the Caged API on POST /v1/sandboxes.
//
// SOURCE OF TRUTH: `caged-api/internal/api/request.go`, in
// `(*CreateSandboxRequest).Validate`. These constants MUST track that
// function. They are duplicated here only because the API exposes no
// machine-readable limits surface — there is no capabilities endpoint, no
// OpenAPI document, and no shared module the CLI can import. The moment
// one exists, fetch it and keep these as the offline fallback (see
// Limits/DefaultLimits below, which exist to make that a one-file change).
//
// The CLI must not invent ceilings of its own. It did once — 16 vCPU,
// 32768 MB, 100 GB — and every value in between passed `caged up` locally
// and was then refused by the server, telling the user their config was
// fine and then contradicting it one HTTP call later.
//
// Zero means "let the server choose the default", which is why the lower
// bound is 0 rather than 1.
const (
	// MaxCPU is the largest accepted `resources.cpu` / `--cpus`.
	MaxCPU = 8
	// MaxMemoryMB is the largest accepted `resources.memory` / `--memory`.
	MaxMemoryMB = 8192
	// MaxDiskGB is the largest accepted `resources.disk` / `--disk`.
	MaxDiskGB = 50
)

// Limits is the set of resource ceilings a config is checked against.
// Today it is always DefaultLimits(); it is a type rather than bare
// constant use so that a server-reported set of limits can be substituted
// without touching the validator.
type Limits struct {
	MaxCPU      int
	MaxMemoryMB int
	MaxDiskGB   int
}

// DefaultLimits returns the ceilings mirrored from the API. It is the
// fallback for any future limits lookup that finds the server unreachable
// or too old to report them.
func DefaultLimits() Limits {
	return Limits{
		MaxCPU:      MaxCPU,
		MaxMemoryMB: MaxMemoryMB,
		MaxDiskGB:   MaxDiskGB,
	}
}
