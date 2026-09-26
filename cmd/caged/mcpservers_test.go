package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/caged-dev/cli/internal/api"
)

// TestBareMCPArgIsStillTheStdioBridge is the compatibility assertion this whole
// dispatch exists for.
//
// `caged mcp <sandbox-id>` is what an MCP client config invokes:
//
//	{"mcpServers":{"caged":{"command":"caged","args":["mcp","cage_abc123"]}}}
//
// Adding management subcommands under the same verb must not shadow it, or every
// claude_desktop_config.json in the field breaks on upgrade.
func TestBareMCPArgIsStillTheStdioBridge(t *testing.T) {
	for _, sandboxID := range []string{
		"cage_abc123", "cage_a1b2c3d4", "0f4c1b2e-0000-0000-0000-000000000001",
	} {
		if isMCPSubcommand(sandboxID) {
			t.Fatalf("%q would be dispatched as a management subcommand, shadowing the stdio bridge",
				sandboxID)
		}
	}
}

func TestEveryDocumentedSubcommandDispatches(t *testing.T) {
	// The list is here rather than derived, so adding a case to the dispatch
	// without documenting it, or documenting one without wiring it, fails.
	for _, sub := range []string{
		"servers", "bind", "unbind", "bindings", "tools", "readiness",
		"allow", "disallow", "catalogue", "catalog", "oauth", "inputs",
		"help", "--help", "-h",
	} {
		if !isMCPSubcommand(sub) {
			t.Fatalf("%q is documented but is not dispatched", sub)
		}
	}
}

// The usage text has to name the two things that otherwise cost an operator an
// afternoon. Asserted, because a help string is the one piece of documentation
// that ships with the binary.
func TestUsageNamesTheTwoTrapsAndEverySubcommand(t *testing.T) {
	usage := captureStdout(t, printMCPUsage)
	for _, want := range []string{
		"does NOT make its tools callable",
		"including autonomous",
		"--allow-tools",
		"readiness",
		"quarantined",
		"tools diff",
		"oauth consent",
		"forwards nothing",
		"<sandbox-id>",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage does not mention %q", want)
		}
	}
}

func TestSplitScopesAcceptsSpacesAndCommas(t *testing.T) {
	for _, raw := range []string{"repo:read issues:write", "repo:read,issues:write", " repo:read , issues:write "} {
		got := splitScopes(raw)
		if len(got) != 2 || got[0] != "repo:read" || got[1] != "issues:write" {
			t.Fatalf("splitScopes(%q) = %v", raw, got)
		}
	}
	if len(splitScopes("")) != 0 {
		t.Fatal("an empty scope string must yield nothing")
	}
}

func TestSplitList(t *testing.T) {
	got := splitList("a, b ,,c")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("splitList = %v", got)
	}
	if splitList("  ") != nil {
		t.Fatal("a blank list must be nil, so it is omitted from the request")
	}
}

func TestWrapTextWrapsAtWordBoundaries(t *testing.T) {
	text := "Caged will send you to https://github.com to authorize access for the scopes listed."
	wrapped := wrapText(text, 30)
	for _, line := range strings.Split(wrapped, "\n") {
		if len(line) > 30 && !strings.Contains(line, "https://github.com") {
			t.Fatalf("line is %d characters: %q", len(line), line)
		}
	}
	if strings.ReplaceAll(wrapped, "\n", " ") != text {
		t.Fatalf("wrapping lost or added words: %q", wrapped)
	}
	if wrapText("", 10) != "" {
		t.Fatal("empty in, empty out")
	}
}

func TestPreviewIsBounded(t *testing.T) {
	if got := preview(nil); got != "" {
		t.Fatalf("preview(nil) = %q", got)
	}
	if got := preview([]string{"a", "b", "c", "d", "e"}); !strings.HasSuffix(got, "…)") {
		t.Fatalf("preview = %q, want it truncated", got)
	}
}

// printPolicyAdvice must offer the allow command when — and only when — the
// remedy applies and the rule is not already there.
func TestPolicyAdviceOffersTheAllowCommandOnlyWhenItHelps(t *testing.T) {
	unclassified := &api.MCPPolicyAdvice{
		Status: "unclassified", Remedy: "allow_mcp_server",
		ToolsEvaluated: 9, Explanation: "nothing classifies its tools",
	}
	out := captureStdout(t, func() { printPolicyAdvice(unclassified, "srv-1", "persona-1") })
	if !strings.Contains(out, "caged mcp allow srv-1 --persona persona-1") {
		t.Fatalf("the advice does not offer the command: %q", out)
	}

	alreadyGranted := &api.MCPPolicyAdvice{
		Status: "allowed", ToolsEvaluated: 9, ToolsAllowed: 9,
		GrantedRule: true, Explanation: "policy allows all 9",
	}
	out = captureStdout(t, func() { printPolicyAdvice(alreadyGranted, "srv-1", "persona-1") })
	if strings.Contains(out, "caged mcp allow") {
		t.Fatalf("an allowed server must not be offered a grant: %q", out)
	}

	// A DENY the operator wrote must not be offered an overwrite.
	deliberate := &api.MCPPolicyAdvice{
		Status: "denied", Remedy: "edit_policy", ToolsEvaluated: 9, ToolsDenied: 9,
		Explanation: "rule \"no-github\" refuses this server's tools",
	}
	out = captureStdout(t, func() { printPolicyAdvice(deliberate, "srv-1", "persona-1") })
	if strings.Contains(out, "caged mcp allow") {
		t.Fatalf("a deliberate deny must not be offered an overwrite: %q", out)
	}

	// Nil advice prints nothing rather than a broken empty block.
	if out := captureStdout(t, func() { printPolicyAdvice(nil, "s", "p") }); out != "" {
		t.Fatalf("nil advice printed %q", out)
	}
}

func TestPrintToolTableNamesTheHeldTools(t *testing.T) {
	out := captureStdout(t, func() {
		printToolTable([]api.MCPTool{
			{NamespacedName: "github__get_issue", State: "active", DefinitionTokensEstimate: 120},
			{NamespacedName: "github__create_issue", State: "quarantined", Flags: []string{"injection"}},
		})
	})
	if !strings.Contains(out, "github__create_issue") || !strings.Contains(out, "quarantined") {
		t.Fatalf("out = %q", out)
	}
	if !strings.Contains(out, "caged mcp tools diff") {
		t.Fatalf("a held tool must point at the diff: %q", out)
	}
	if !strings.Contains(out, "estimate") {
		t.Fatalf("the token column must be named an estimate: %q", out)
	}

	empty := captureStdout(t, func() { printToolTable(nil) })
	if !strings.Contains(empty, "refresh") {
		t.Fatalf("an empty catalogue must say what to do: %q", empty)
	}
}

func TestIndentJSONSurvivesAStrangersDocument(t *testing.T) {
	if got := indentJSON([]byte(`{"a":1}`)); !strings.Contains(got, `"a": 1`) {
		t.Fatalf("indentJSON = %q", got)
	}
	// A malformed schema from a third party is printed raw rather than
	// swallowed: a reviewer needs to see it either way.
	if got := indentJSON([]byte(`{not json`)); !strings.Contains(got, "not json") {
		t.Fatalf("indentJSON = %q", got)
	}
}

func TestUnknownSubcommandNamesItself(t *testing.T) {
	err := cmdMCPServers([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
	if err := cmdMCPServersGroup([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
	if err := cmdMCPOAuth([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
	if err := cmdMCPOAuth(nil); err == nil {
		t.Fatal("no oauth subcommand must be a usage error")
	}
}

// Usage errors are returned rather than exiting, so a caller can add a sentence.
func TestUsageErrorsAreReturnedNotExited(t *testing.T) {
	for name, call := range map[string]func() error{
		"bind with no server":    func() error { return cmdMCPBind(nil) },
		"unbind with no id":      func() error { return cmdMCPUnbind(nil) },
		"allow with no server":   func() error { return cmdMCPAllow(nil) },
		"tools diff with no arg": func() error { return cmdMCPToolDiff(nil) },
		"servers rm with no id":  func() error { return cmdMCPServerRemove(nil) },
		"respond with no id":     func() error { return cmdMCPInputRespond(nil) },
	} {
		if err := call(); err == nil {
			t.Fatalf("%s: expected a usage error", name)
		}
	}
}

// An answer that is not valid JSON is refused before anything is sent, because
// the server would reject it and the round-trip is wasted.
func TestRespondRefusesMalformedAnswerJSON(t *testing.T) {
	err := cmdMCPInputRespond([]string{"--answer", "team=not json", "7b21"})
	if err == nil {
		t.Fatal("a malformed answer must be refused locally")
	}
}

func TestRespondRefusesNeitherAnswerNorDecline(t *testing.T) {
	err := cmdMCPInputRespond([]string{"7b21"})
	if err == nil || !strings.Contains(err.Error(), "decline") {
		t.Fatalf("err = %v", err)
	}
}

// captureStdout runs f with os.Stdout redirected and returns what it printed.
//
// These commands communicate by printing, and the two traps this surface exists
// to surface are sentences. A test that could not read them would be testing
// the plumbing and not the point.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	f()
	_ = w.Close()
	os.Stdout = original
	return <-done
}
