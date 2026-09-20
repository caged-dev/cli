package cagefile

import (
	"strings"
	"testing"
)

// TestValidate_ResourceCeilings pins the CLI's ceilings to the API's. Each
// dimension is checked at the value just under the limit, at the limit, and
// just over it — the boundary is where the two validators drifted apart, so
// the boundary is what the test has to hold.
//
// If one of these fails after an API change, the fix is to update
// limits.go to match `caged-api/internal/api/request.go`, not to relax the
// test.
func TestValidate_ResourceCeilings(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		// CPU: API rejects > 8.
		{"cpu just under limit", Config{Template: "node", Resources: Resources{CPU: MaxCPU - 1}}, ""},
		{"cpu at limit", Config{Template: "node", Resources: Resources{CPU: MaxCPU}}, ""},
		{"cpu just over limit", Config{Template: "node", Resources: Resources{CPU: MaxCPU + 1}}, "resources.cpu must be 0-8"},
		{"cpu zero means default", Config{Template: "node", Resources: Resources{CPU: 0}}, ""},
		{"cpu negative", Config{Template: "node", Resources: Resources{CPU: -1}}, "resources.cpu must be 0-8"},
		// The old CLI-only ceiling: accepted locally, refused by the API.
		{"cpu at old CLI ceiling", Config{Template: "node", Resources: Resources{CPU: 16}}, "resources.cpu must be 0-8"},

		// Memory: API rejects > 8192 MB.
		{"memory just under limit", Config{Template: "node", Resources: Resources{Memory: MaxMemoryMB - 1}}, ""},
		{"memory at limit", Config{Template: "node", Resources: Resources{Memory: MaxMemoryMB}}, ""},
		{"memory just over limit", Config{Template: "node", Resources: Resources{Memory: MaxMemoryMB + 1}}, "resources.memory must be 0-8192"},
		{"memory zero means default", Config{Template: "node", Resources: Resources{Memory: 0}}, ""},
		{"memory negative", Config{Template: "node", Resources: Resources{Memory: -1}}, "resources.memory must be 0-8192"},
		{"memory at old CLI ceiling", Config{Template: "node", Resources: Resources{Memory: 32768}}, "resources.memory must be 0-8192"},

		// Disk: API rejects > 50 GB.
		{"disk just under limit", Config{Template: "node", Resources: Resources{Disk: MaxDiskGB - 1}}, ""},
		{"disk at limit", Config{Template: "node", Resources: Resources{Disk: MaxDiskGB}}, ""},
		{"disk just over limit", Config{Template: "node", Resources: Resources{Disk: MaxDiskGB + 1}}, "resources.disk must be 0-50"},
		{"disk zero means default", Config{Template: "node", Resources: Resources{Disk: 0}}, ""},
		{"disk negative", Config{Template: "node", Resources: Resources{Disk: -1}}, "resources.disk must be 0-50"},
		{"disk at old CLI ceiling", Config{Template: "node", Resources: Resources{Disk: 100}}, "resources.disk must be 0-50"},

		// A config that used to pass `caged up` and then fail the create
		// call, on every dimension at once.
		{
			"all three between the old CLI ceiling and the API's",
			Config{Template: "node", Resources: Resources{CPU: 12, Memory: 16384, Disk: 80}},
			"resources.cpu must be 0-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// TestDefaultLimits guards the mirrored numbers themselves. They must equal
// the bounds in `caged-api/internal/api/request.go`; a change to either
// side without the other is the bug this whole file exists to prevent.
func TestDefaultLimits(t *testing.T) {
	got := DefaultLimits()
	want := Limits{MaxCPU: 8, MaxMemoryMB: 8192, MaxDiskGB: 50}
	if got != want {
		t.Errorf("DefaultLimits() = %+v, want %+v (must match CreateSandboxRequest.Validate)", got, want)
	}
}

// TestValidateWithLimits covers the seam a server-reported limits source
// would use: the same validator, different ceilings.
func TestValidateWithLimits(t *testing.T) {
	tests := []struct {
		name    string
		limits  Limits
		cfg     Config
		wantErr bool
	}{
		{
			name:   "server allows more than the mirrored default",
			limits: Limits{MaxCPU: 32, MaxMemoryMB: 65536, MaxDiskGB: 200},
			cfg:    Config{Template: "node", Resources: Resources{CPU: 16, Memory: 32768, Disk: 100}},
		},
		{
			name:    "server allows less than the mirrored default",
			limits:  Limits{MaxCPU: 2, MaxMemoryMB: 1024, MaxDiskGB: 10},
			cfg:     Config{Template: "node", Resources: Resources{CPU: 4}},
			wantErr: true,
		},
		{
			name:   "at the supplied limit exactly",
			limits: Limits{MaxCPU: 2, MaxMemoryMB: 1024, MaxDiskGB: 10},
			cfg:    Config{Template: "node", Resources: Resources{CPU: 2, Memory: 1024, Disk: 10}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.ValidateWithLimits(tt.limits)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateWithLimits(%+v) = nil, want error", tt.limits)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateWithLimits(%+v) = %v, want nil", tt.limits, err)
			}
		})
	}
}

// TestValidate_EnvKeys mirrors the API's env-key rule: a key containing
// '=', a space, a tab or a newline is refused there, so refusing it here
// saves a create round trip that could only end in a 400.
func TestValidate_EnvKeys(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"plain key", map[string]string{"API_KEY": "v"}, ""},
		{"key with dots and dashes", map[string]string{"my.key-1": "v"}, ""},
		{"empty key is the API's business", map[string]string{"": "v"}, ""},
		{"key with equals", map[string]string{"A=B": "v"}, `invalid env key "A=B"`},
		{"key with space", map[string]string{"A B": "v"}, `invalid env key "A B"`},
		{"key with tab", map[string]string{"A\tB": "v"}, `invalid env key "A\tB"`},
		{"key with newline", map[string]string{"A\nB": "v"}, `invalid env key "A\nB"`},
		{"one bad key among good ones", map[string]string{"OK": "v", "BAD KEY": "v", "ALSO_OK": "v"}, `invalid env key "BAD KEY"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Template: "node", Env: tt.env}
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_EnvKeyErrorsAreOrdered keeps the message reproducible when
// several keys are bad: map iteration order must not leak into output a
// user reads or a test asserts on.
func TestValidate_EnvKeyErrorsAreOrdered(t *testing.T) {
	cfg := Config{Template: "node", Env: map[string]string{
		"C D": "v", "A B": "v", "B C": "v",
	}}
	for range 20 {
		err := cfg.Validate()
		if err == nil {
			t.Fatal("Validate() = nil, want error")
		}
		got := err.Error()
		a := strings.Index(got, `"A B"`)
		b := strings.Index(got, `"B C"`)
		c := strings.Index(got, `"C D"`)
		if a < 0 || b < 0 || c < 0 || a >= b || b >= c {
			t.Fatalf("env key errors not in sorted order: %q", got)
		}
	}
}
