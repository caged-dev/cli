package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/caged-dev/cli/internal/api"
)

// `caged mcp servers|bind|unbind|tools|readiness|allow|oauth|inputs` — managing
// the third-party MCP servers an agent in a sandbox can reach.
//
// Two things about this surface are worth stating, because getting either wrong
// wastes an operator's afternoon and both are invisible from the API's happy
// path:
//
//  1. **Binding a server does not make its tools callable.** Caged's
//     autonomy-tier table classifies its OWN tool names; `github__get_issue`
//     matches none of them, so a bound external tool is denied at every tier —
//     including `autonomous` — until a policy rule allows it. Every command
//     here that can surface that fact does, and `bind --allow-tools` writes the
//     rule in one step.
//  2. **A quarantined tool is a definition that CHANGED.** `tools` prints the
//     state, and `tools diff` shows the definition a human approved next to the
//     one the server is advertising now. Approving without reading the diff is a
//     consent dialog with the text removed.

// cmdMCPServers dispatches everything under `caged mcp` that is not the stdio
// bridge.
//
// The bridge keeps the bare form — `caged mcp <sandbox-id>` — because that is
// what an MCP client config invokes, and breaking it would break every
// `claude_desktop_config.json` in the field. A subcommand name is never a valid
// sandbox id (they are `cage_…`), so the two cannot collide.
func cmdMCPServers(args []string) error {
	switch args[0] {
	case "servers":
		return cmdMCPServersGroup(args[1:])
	case "bind":
		return cmdMCPBind(args[1:])
	case "unbind":
		return cmdMCPUnbind(args[1:])
	case "bindings":
		return cmdMCPBindings(args[1:])
	case "tools":
		return cmdMCPTools(args[1:])
	case "readiness":
		return cmdMCPReadiness(args[1:])
	case "allow":
		return cmdMCPAllow(args[1:])
	case "disallow":
		return cmdMCPDisallow(args[1:])
	case "catalogue", "catalog":
		return cmdMCPCatalogue(args[1:])
	case "oauth":
		return cmdMCPOAuth(args[1:])
	case "inputs":
		return cmdMCPInputs(args[1:])
	default:
		return fmt.Errorf("unknown mcp command: %s (run 'caged mcp help' for usage)", args[0])
	}
}

// isMCPSubcommand reports whether the first argument names a management
// subcommand rather than a sandbox id.
func isMCPSubcommand(arg string) bool {
	switch arg {
	case "servers", "bind", "unbind", "bindings", "tools", "readiness",
		"allow", "disallow", "catalogue", "catalog", "oauth", "inputs",
		"help", "--help", "-h":
		return true
	default:
		return false
	}
}

func cmdMCPServersGroup(args []string) error {
	if len(args) == 0 {
		printMCPUsage()
		return nil
	}
	switch args[0] {
	case "add", "register":
		return cmdMCPServerAdd(args[1:])
	case "list", "ls":
		return cmdMCPServerList(args[1:])
	case "show", "get":
		return cmdMCPServerShow(args[1:])
	case "rm", "remove", "delete":
		return cmdMCPServerRemove(args[1:])
	case "refresh":
		return cmdMCPServerRefresh(args[1:])
	default:
		return fmt.Errorf("unknown mcp servers command: %s", args[0])
	}
}

// --- servers add -------------------------------------------------------

func cmdMCPServerAdd(args []string) error {
	fs := newFlagSet("mcp servers add")
	var (
		alias       = fs.String("alias", "", "the alias tools are namespaced under: alias__tool")
		catalogueID = fs.String("catalogue-id", "", "register from Caged's reviewed catalogue")
		endpoint    = fs.String("endpoint", "", "the server's https Streamable HTTP endpoint")
		displayName = fs.String("name", "", "display name (defaults to the alias)")
		description = fs.String("description", "", "description")
		authKind    = fs.String("auth", "", "none | bearer | header | oauth")
		token       = fs.String("token", "", "the bearer token; or set CAGED_MCP_TOKEN")
		header      = fs.String("header", "", "a header credential as Name:Value (repeatable)")
		asJSON      = fs.Bool("json", false, "print the registration as JSON")
	)
	var headers []string
	fs.Func("header-set", "a header credential as Name:Value (repeatable)", func(v string) error {
		headers = append(headers, v)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *header != "" {
		headers = append(headers, *header)
	}

	req := &api.CreateMCPServerRequest{
		CatalogueID: *catalogueID,
		Alias:       *alias,
		DisplayName: *displayName,
		Description: *description,
		Endpoint:    *endpoint,
		AuthKind:    *authKind,
	}
	if *catalogueID == "" && *endpoint == "" {
		return fmt.Errorf("one of --catalogue-id or --endpoint is required " +
			"(run 'caged mcp catalogue' for the reviewed servers)")
	}

	// The token is read from the environment in preference to the flag, so it
	// does not land in a shell history file.
	req.Credential = firstNonEmptyStr(os.Getenv("CAGED_MCP_TOKEN"), *token)
	if len(headers) > 0 {
		req.Headers = map[string]string{}
		for _, raw := range headers {
			name, value, ok := strings.Cut(raw, ":")
			if !ok {
				return fmt.Errorf("--header must be Name:Value, got %q", raw)
			}
			req.Headers[strings.TrimSpace(name)] = strings.TrimSpace(value)
		}
		if req.AuthKind == "" {
			req.AuthKind = "header"
		}
	}
	if req.Credential != "" && req.AuthKind == "" {
		req.AuthKind = "bearer"
	}

	client, err := mustClient()
	if err != nil {
		return err
	}
	srv, err := client.CreateMCPServer(context.Background(), req)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(srv)
	}

	fmt.Printf("registered %s (%s)\n", srv.Alias, srv.ID)
	fmt.Printf("  endpoint   %s\n", srv.Endpoint)
	fmt.Printf("  transport  %s\n", srv.Transport)
	fmt.Printf("  auth       %s\n", srv.AuthKind)
	fmt.Printf("  verified   %v\n", srv.Verified)
	fmt.Printf("  status     %s\n", srv.Status)
	if srv.OAuthNextStep != "" {
		fmt.Printf("\nthis server needs authorizing before any agent can use it:\n")
		fmt.Printf("  caged mcp oauth show %s\n", srv.ID)
		fmt.Printf("  caged mcp oauth consent %s --persona <id>\n", srv.ID)
		fmt.Printf("  caged mcp oauth authorize %s --persona <id>\n", srv.ID)
	}
	fmt.Printf("\nnext:\n")
	fmt.Printf("  caged mcp servers refresh %s      # fetch and pin its tool catalogue\n", srv.ID)
	fmt.Printf("  caged mcp bind %s --persona <id>  # make it visible to a persona\n", srv.ID)
	fmt.Printf("\nregistering a server makes it KNOWN. It is visible to nothing until it is bound,\n")
	fmt.Printf("and its tools are denied by policy until a rule allows them — 'caged mcp bind'\n")
	fmt.Printf("will tell you which, and '--allow-tools' writes it.\n")
	return nil
}

// --- servers list / show / rm / refresh --------------------------------

func cmdMCPServerList(args []string) error {
	fs := newFlagSet("mcp servers list")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	servers, err := client.ListMCPServers(context.Background())
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(servers)
	}
	if len(servers) == 0 {
		fmt.Println("no MCP servers registered")
		fmt.Println("\n  caged mcp catalogue                  # the servers Caged has reviewed")
		fmt.Println("  caged mcp servers add --alias github --catalogue-id <id> --token <t>")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "ALIAS\tSTATUS\tAUTH\tVERIFIED\tID\tENDPOINT")
	for _, srv := range servers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\t%s\n",
			srv.Alias, srv.Status, srv.AuthKind, srv.Verified, srv.ID, srv.Endpoint)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Println("\n'verified false' means an arbitrary URL rather than Caged's reviewed catalogue.")
	fmt.Println("That word also reaches the agent's model in the tool framing.")
	return nil
}

func cmdMCPServerShow(args []string) error {
	fs := newFlagSet("mcp servers show")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp servers show <server-id>")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	detail, err := client.GetMCPServer(context.Background(), fs.Arg(0))
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(detail)
	}
	fmt.Printf("%s (%s)\n", detail.Alias, detail.ID)
	fmt.Printf("  endpoint   %s\n", detail.Endpoint)
	fmt.Printf("  transport  %s\n", detail.Transport)
	fmt.Printf("  auth       %s\n", detail.AuthKind)
	fmt.Printf("  protocol   %s %s\n", detail.ProtocolEra, detail.ProtocolVersion)
	fmt.Printf("  verified   %v\n", detail.Verified)
	fmt.Printf("  status     %s\n", detail.Status)
	if detail.QuarantineReason != "" {
		fmt.Printf("  quarantine %s\n", detail.QuarantineReason)
	}
	fmt.Println()
	printToolTable(detail.Tools)
	return nil
}

func cmdMCPServerRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: caged mcp servers rm <server-id>")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	if err := client.DeleteMCPServer(context.Background(), args[0]); err != nil {
		return err
	}
	fmt.Printf("deregistered %s — its catalogue and every binding to it are gone\n", args[0])
	return nil
}

func cmdMCPServerRefresh(args []string) error {
	fs := newFlagSet("mcp servers refresh")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp servers refresh <server-id>")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	report, err := client.RefreshMCPServer(context.Background(), fs.Arg(0))
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(report)
	}
	fmt.Printf("refreshed %s (%s %s)\n", report.Alias, report.Era, report.Version)
	fmt.Printf("  added       %d %s\n", len(report.Added), preview(report.Added))
	fmt.Printf("  unchanged   %d\n", len(report.Unchanged))
	fmt.Printf("  changed     %d %s\n", len(report.Changed), preview(report.Changed))
	fmt.Printf("  withdrawn   %d %s\n", len(report.Withdrawn), preview(report.Withdrawn))
	fmt.Printf("  quarantined %d %s\n", len(report.Quarantined), preview(report.Quarantined))
	if len(report.Quarantined) > 0 {
		fmt.Printf("\na quarantined tool's definition CHANGED since it was approved. It is not callable\n")
		fmt.Printf("and it is not advertised to any agent until a human decides. Read the change first:\n")
		for _, tool := range report.Quarantined {
			fmt.Printf("  caged mcp tools diff %s %s\n", fs.Arg(0), tool)
		}
	}
	return nil
}

// --- bind / unbind / bindings ------------------------------------------

func cmdMCPBind(args []string) error {
	fs := newFlagSet("mcp bind")
	var (
		persona    = fs.String("persona", "", "the persona to bind to; omit for an account-wide binding")
		tools      = fs.String("tools", "", "comma-separated upstream tool names; empty means every approved tool")
		deny       = fs.String("deny", "", "comma-separated upstream tool names to exclude")
		pinned     = fs.Bool("pinned", false, "never defer these tools behind the token budget")
		ceiling    = fs.Int("argument-ceiling", 0, "narrow the per-call argument ceiling, in bytes")
		allowTools = fs.Bool("allow-tools", false, "also write the policy rule that makes these tools callable")
		asJSON     = fs.Bool("json", false, "print as JSON")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp bind <server-id> [--persona <id>] [--allow-tools]")
	}

	req := &api.CreateMCPBindingRequest{
		ServerID:             fs.Arg(0),
		SubjectKind:          "account",
		ToolAllowlist:        splitList(*tools),
		ToolDenylist:         splitList(*deny),
		Pinned:               *pinned,
		ArgumentCeilingBytes: *ceiling,
		AllowTools:           *allowTools,
	}
	if *persona != "" {
		req.SubjectKind = "persona"
		req.SubjectID = *persona
	}

	client, err := mustClient()
	if err != nil {
		return err
	}
	result, err := client.CreateMCPBinding(context.Background(), req)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(result)
	}

	fmt.Printf("bound %s to %s", req.ServerID, req.SubjectKind)
	if req.SubjectID != "" {
		fmt.Printf(" %s", req.SubjectID)
	}
	fmt.Println()
	if len(result.ToolAllowlist) > 0 {
		fmt.Printf("  tools     %s\n", strings.Join(result.ToolAllowlist, ", "))
	} else {
		fmt.Printf("  tools     every approved tool on the server\n")
	}
	if result.Pinned {
		fmt.Printf("  pinned    yes (never deferred behind the token budget)\n")
	}

	if result.PolicyRuleWritten != nil {
		g := result.PolicyRuleWritten
		fmt.Printf("\nwrote the policy allow rule %s for %s\n", g.RuleID, g.ToolPattern)
		if g.PolicyCreated {
			fmt.Printf("  this persona had no stored policy, so Caged created one as a copy of the\n")
			fmt.Printf("  %s tier template plus that rule. Nothing else about its policy changed.\n", g.TierTemplateID)
		}
		if g.AlreadyPresent {
			fmt.Printf("  (the rule was already there; nothing changed)\n")
		}
	} else if result.PolicyRuleError != "" {
		fmt.Printf("\nthe binding exists, but the allow rule was not written:\n  %s\n", result.PolicyRuleError)
	}

	printPolicyAdvice(result.PolicyAdvice, req.ServerID, req.SubjectID)
	return nil
}

func cmdMCPUnbind(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: caged mcp unbind <binding-id>\n" +
			"  run 'caged mcp bindings' for the ids")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	if err := client.DeleteMCPBinding(context.Background(), args[0]); err != nil {
		return err
	}
	fmt.Printf("unbound %s — it takes effect on the agent's next tools/list, not after a cache expiry\n", args[0])
	return nil
}

func cmdMCPBindings(args []string) error {
	fs := newFlagSet("mcp bindings")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	bindings, err := client.ListMCPBindings(context.Background())
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(bindings)
	}
	if len(bindings) == 0 {
		fmt.Println("no bindings — every agent's external tool set is empty, which is the default")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "BINDING\tSERVER\tSUBJECT\tPINNED\tTOOLS")
	for _, b := range bindings {
		subject := b.SubjectKind
		if b.SubjectID != "" {
			subject += " " + b.SubjectID
		}
		tools := "all approved"
		if len(b.ToolAllowlist) > 0 {
			tools = strings.Join(b.ToolAllowlist, ",")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\n", b.ID, b.ServerID, subject, b.Pinned, tools)
	}
	return w.Flush()
}

// --- tools -------------------------------------------------------------

func cmdMCPTools(args []string) error {
	if len(args) > 0 && args[0] == "diff" {
		return cmdMCPToolDiff(args[1:])
	}
	if len(args) > 0 && args[0] == "approve" {
		return cmdMCPToolDecide(args[1:], true)
	}
	if len(args) > 0 && args[0] == "reject" {
		return cmdMCPToolDecide(args[1:], false)
	}

	fs := newFlagSet("mcp tools")
	var (
		asJSON = fs.Bool("json", false, "print as JSON")
		state  = fs.String("state", "", "only tools in this state: pending, active, quarantined, withdrawn")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	// With no server named, every registered server's catalogue: the question
	// "what can my agents actually reach?" is about all of them.
	var servers []string
	if fs.NArg() >= 1 {
		servers = []string{fs.Arg(0)}
	} else {
		registered, err := client.ListMCPServers(ctx)
		if err != nil {
			return err
		}
		for _, srv := range registered {
			servers = append(servers, srv.ID)
		}
		if len(servers) == 0 {
			fmt.Println("no MCP servers registered")
			return nil
		}
	}

	var all []api.MCPTool
	for _, id := range servers {
		detail, err := client.GetMCPServer(ctx, id)
		if err != nil {
			return err
		}
		for _, tool := range detail.Tools {
			if *state != "" && tool.State != *state {
				continue
			}
			all = append(all, tool)
		}
	}
	if *asJSON {
		return printJSON(all)
	}
	printToolTable(all)
	return nil
}

func printToolTable(tools []api.MCPTool) {
	if len(tools) == 0 {
		fmt.Println("no tools in the pinned catalogue — run 'caged mcp servers refresh <id>'")
		return
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].NamespacedName < tools[j].NamespacedName })
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "TOOL (as the agent calls it)\tSTATE\tTOKENS~\tFLAGS")
	held := 0
	for _, tool := range tools {
		if tool.State != "active" {
			held++
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
			tool.NamespacedName, tool.State, tool.DefinitionTokensEstimate, strings.Join(tool.Flags, ","))
	}
	_ = w.Flush()
	fmt.Println("\nTOKENS~ is an estimate: Go has no first-party BPE, so this is a measured heuristic.")
	if held > 0 {
		fmt.Printf("%d tool(s) are not 'active', so they are advertised to no agent and fail the gate\n", held)
		fmt.Println("if called anyway. 'quarantined' means the definition CHANGED — read it before deciding:")
		fmt.Println("  caged mcp tools diff <server-id> <tool>")
	}
}

func cmdMCPToolDiff(args []string) error {
	fs := newFlagSet("mcp tools diff")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: caged mcp tools diff <server-id> <tool>")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	diff, err := client.MCPToolDiffFor(context.Background(), fs.Arg(0), fs.Arg(1))
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(diff)
	}

	fmt.Printf("%s  (state: %s)\n\n", diff.NamespacedName, diff.State)
	fmt.Printf("%s\n\n", diff.Explanation)
	if len(diff.AddedProperties) > 0 {
		fmt.Printf("  + parameters  %s\n", strings.Join(diff.AddedProperties, ", "))
	}
	if len(diff.RemovedProperties) > 0 {
		fmt.Printf("  - parameters  %s\n", strings.Join(diff.RemovedProperties, ", "))
	}
	if len(diff.Changed) > 0 {
		fmt.Printf("  ~ changed     %s\n", strings.Join(diff.Changed, ", "))
	}
	if diff.Approved != nil {
		fmt.Printf("\n--- approved (%s)\n%s\n", diff.ApprovedDigest, diff.Approved.Description)
		if len(diff.Approved.InputSchema) > 0 {
			fmt.Printf("%s\n", indentJSON(diff.Approved.InputSchema))
		}
	}
	if diff.Current != nil {
		fmt.Printf("\n+++ current (%s)\n%s\n", diff.CurrentDigest, diff.Current.Description)
		if len(diff.Current.InputSchema) > 0 {
			fmt.Printf("%s\n", indentJSON(diff.Current.InputSchema))
		}
		if len(diff.Current.Flags) > 0 {
			fmt.Printf("\nCaged's metadata scan flags this definition: %s\n",
				strings.Join(diff.Current.Flags, ", "))
			fmt.Printf("A flagged definition cannot be approved in one step, and that is deliberate.\n")
		}
	}
	fmt.Printf("\n  caged mcp tools approve %s %s\n", fs.Arg(0), fs.Arg(1))
	fmt.Printf("  caged mcp tools reject  %s %s --note \"...\"\n", fs.Arg(0), fs.Arg(1))
	return nil
}

func cmdMCPToolDecide(args []string, approve bool) error {
	fs := newFlagSet("mcp tools decide")
	note := fs.String("note", "", "why (recorded against this exact definition)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		verb := "approve"
		if !approve {
			verb = "reject"
		}
		return fmt.Errorf("usage: caged mcp tools %s <server-id> <tool>", verb)
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	ctx := context.Background()
	if approve {
		if err := client.ApproveMCPTool(ctx, fs.Arg(0), fs.Arg(1)); err != nil {
			return err
		}
		fmt.Printf("approved %s — it is advertised again on the agent's next tools/list\n", fs.Arg(1))
		return nil
	}
	if err := client.RejectMCPTool(ctx, fs.Arg(0), fs.Arg(1), *note); err != nil {
		return err
	}
	fmt.Printf("rejected %s — it stays unavailable to every agent.\n", fs.Arg(1))
	fmt.Printf("The refusal is recorded against this exact definition, so the same bytes will not\n")
	fmt.Printf("come back as a new review.\n")
	return nil
}

// --- readiness / allow -------------------------------------------------

func cmdMCPReadiness(args []string) error {
	fs := newFlagSet("mcp readiness")
	var (
		persona = fs.String("persona", "", "the persona to answer for")
		asJSON  = fs.Bool("json", false, "print as JSON")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	advice, err := client.MCPReadiness(context.Background(), *persona)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(advice)
	}
	if len(advice) == 0 {
		fmt.Println("no servers are bound to this subject")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "ALIAS\tSTATUS\tALLOWED/TOTAL\tREMEDY")
	blocked := 0
	for _, a := range advice {
		if a.Status != "allowed" && a.Status != "needs_approval" {
			blocked++
		}
		fmt.Fprintf(w, "%s\t%s\t%d/%d\t%s\n",
			a.Alias, a.Status, a.ToolsAllowed, a.ToolsEvaluated, a.Remedy)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Println("\nAn external MCP tool is denied until a policy rule allows it. Caged cannot tell a")
	fmt.Println("read-only third-party tool from a destructive one, so an unclassified one is refused")
	fmt.Println("at every autonomy tier — including autonomous.")
	if blocked > 0 {
		fmt.Printf("\n%d server(s) need attention:\n\n", blocked)
		for _, a := range advice {
			if a.Status == "allowed" || a.Status == "needs_approval" {
				continue
			}
			fmt.Printf("  %s — %s\n", a.Alias, a.Explanation)
			if a.Remedy == "allow_mcp_server" && *persona != "" {
				fmt.Printf("      caged mcp allow %s --persona %s\n", a.ServerID, *persona)
			}
			fmt.Println()
		}
	}
	return nil
}

func cmdMCPAllow(args []string) error {
	fs := newFlagSet("mcp allow")
	persona := fs.String("persona", "", "the persona whose policy the rule goes on")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp allow <server-id> --persona <id>")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	result, err := client.AllowMCPServer(context.Background(), fs.Arg(0), *persona)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(result)
	}
	if result.AlreadyPresent {
		fmt.Printf("%s was already allowed for this persona; nothing changed\n", result.ToolPattern)
		return nil
	}
	fmt.Printf("allowed %s for this persona (rule %s)\n", result.ToolPattern, result.RuleID)
	if result.PolicyCreated {
		fmt.Printf("\nThis persona had no stored policy, so Caged created one as an exact copy of the\n")
		fmt.Printf("%s tier template plus that one rule. Every guardrail the template carries is\n", result.TierTemplateID)
		fmt.Printf("still there — a policy containing only the allow rule would have dropped them.\n")
	}
	fmt.Printf("\nThe rule sits above the catch-all deny and BELOW every always-on guardrail, so it\n")
	fmt.Printf("cannot override the secret-path or private-network rules.\n")
	return nil
}

func cmdMCPDisallow(args []string) error {
	fs := newFlagSet("mcp disallow")
	persona := fs.String("persona", "", "the persona whose policy the rule is on")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp disallow <server-id> --persona <id>")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	if err := client.DisallowMCPServer(context.Background(), fs.Arg(0), *persona); err != nil {
		return err
	}
	fmt.Println("removed the rule Caged wrote. A rule you wrote yourself is untouched.")
	return nil
}

// --- catalogue ---------------------------------------------------------

func cmdMCPCatalogue(args []string) error {
	fs := newFlagSet("mcp catalogue")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	entries, err := client.ListMCPCatalogue(context.Background())
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(entries)
	}
	if len(entries) == 0 {
		fmt.Println("Caged's reviewed catalogue is empty in this deployment")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "CATALOGUE ID\tALIAS\tAUTH\tNAME")
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.ID, e.DefaultAlias, e.AuthKind, e.DisplayName)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Println("\nA server registered from this catalogue is VERIFIED, and its tools arrive usable.")
	fmt.Println("A server registered from an arbitrary URL is not, and its tools are held for review.")
	return nil
}

// --- oauth -------------------------------------------------------------

func cmdMCPOAuth(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: caged mcp oauth <show|consent|authorize> <server-id>")
	}
	switch args[0] {
	case "show", "status":
		return cmdMCPOAuthShow(args[1:])
	case "consent":
		return cmdMCPOAuthConsent(args[1:])
	case "authorize", "auth":
		return cmdMCPOAuthAuthorize(args[1:])
	default:
		return fmt.Errorf("unknown mcp oauth command: %s", args[0])
	}
}

func cmdMCPOAuthShow(args []string) error {
	fs := newFlagSet("mcp oauth show")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp oauth show <server-id>")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	state, err := client.GetMCPOAuth(context.Background(), fs.Arg(0))
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(state)
	}
	fmt.Printf("authorized  %v\n", state.Status.Authorized)
	if state.Status.Authorized {
		fmt.Printf("  issuer     %s\n", state.Status.Issuer)
		fmt.Printf("  scopes     %s\n", strings.Join(state.Status.Scopes, " "))
		fmt.Printf("  expires    %s\n", state.Status.ExpiresAt)
		fmt.Printf("  refresh    %v\n", state.Status.HasRefresh)
		if state.Status.Expired {
			fmt.Printf("  EXPIRED — re-authorize\n")
		}
	}
	fmt.Printf("\nconsents recorded: %d\n", len(state.Consents))
	for _, consent := range state.Consents {
		subject := "account-wide"
		if consent.PersonaID != "" {
			subject = "persona " + consent.PersonaID
		}
		status := "live"
		if consent.RevokedAt != nil {
			status = "revoked " + *consent.RevokedAt
		}
		fmt.Printf("  %s  %s  %s  [%s]\n", consent.ID, subject, strings.Join(consent.Scopes, " "), status)
	}
	if state.DiscoveryError != "" {
		fmt.Printf("\ndiscovery failed: %s\n", state.DiscoveryError)
		return nil
	}
	if state.Prospect == nil {
		return nil
	}
	fmt.Printf("\nauthorizing would involve:\n")
	fmt.Printf("  issuer     %s\n", state.Prospect.Issuer)
	fmt.Printf("  resource   %s\n", state.Prospect.Resource)
	fmt.Printf("  scopes     %s\n", strings.Join(state.Prospect.Scopes, " "))
	fmt.Printf("\n%s\n", wrapText(state.Prospect.ConsentStatement, 78))
	fmt.Printf("\n  caged mcp oauth consent %s --persona <id> --scopes \"%s\"\n",
		fs.Arg(0), strings.Join(state.Prospect.Scopes, " "))
	return nil
}

func cmdMCPOAuthConsent(args []string) error {
	fs := newFlagSet("mcp oauth consent")
	var (
		persona = fs.String("persona", "", "the persona this consent is for; omit for account-wide")
		issuer  = fs.String("issuer", "", "the authorization server (defaults to the discovered one)")
		scopes  = fs.String("scopes", "", "space- or comma-separated scopes")
		yes     = fs.Bool("yes", false, "skip the confirmation prompt")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp oauth consent <server-id> --persona <id> --scopes \"...\"")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	wantIssuer := *issuer
	wantScopes := splitScopes(*scopes)
	if wantIssuer == "" || len(wantScopes) == 0 {
		state, err := client.GetMCPOAuth(ctx, fs.Arg(0))
		if err != nil {
			return err
		}
		if state.Prospect == nil {
			return fmt.Errorf("this server's authorization server could not be discovered, so there is "+
				"nothing to consent to yet: %s", state.DiscoveryError)
		}
		if wantIssuer == "" {
			wantIssuer = state.Prospect.Issuer
		}
		if len(wantScopes) == 0 {
			wantScopes = state.Prospect.Scopes
		}
		// Printed before it is recorded, always. A consent recorded from a
		// discovered value the operator never saw is not a consent.
		fmt.Printf("%s\n\n", wrapText(state.Prospect.ConsentStatement, 78))
	}
	fmt.Printf("issuer  %s\nscopes  %s\n", wantIssuer, strings.Join(wantScopes, " "))
	if *persona != "" {
		fmt.Printf("persona %s\n", *persona)
	} else {
		fmt.Printf("persona (none) — this consent will apply to every persona on the account\n")
	}
	if !*yes && !confirm("record this consent?") {
		return fmt.Errorf("cancelled; nothing was recorded and nothing was sent to %s", wantIssuer)
	}

	if err := client.RecordMCPOAuthConsent(ctx, fs.Arg(0), *persona, wantIssuer, wantScopes); err != nil {
		return err
	}
	fmt.Printf("\nrecorded. Nothing has been sent to %s yet.\n", wantIssuer)
	fmt.Printf("  caged mcp oauth authorize %s", fs.Arg(0))
	if *persona != "" {
		fmt.Printf(" --persona %s", *persona)
	}
	fmt.Println()
	return nil
}

func cmdMCPOAuthAuthorize(args []string) error {
	fs := newFlagSet("mcp oauth authorize")
	persona := fs.String("persona", "", "the persona whose consent to use")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp oauth authorize <server-id> [--persona <id>]")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	authURL, err := client.StartMCPOAuth(context.Background(), fs.Arg(0), *persona)
	if err != nil {
		return err
	}
	fmt.Printf("Open this URL to authorize:\n\n%s\n\n", authURL)
	fmt.Printf("It is single-use and expires in 10 minutes.\n")
	fmt.Printf("Caged holds the resulting token itself: sealed at rest, never written into a\n")
	fmt.Printf("sandbox, never in an environment variable, and never returned by any API read.\n")
	return nil
}

// --- inputs ------------------------------------------------------------

func cmdMCPInputs(args []string) error {
	if len(args) > 0 && args[0] == "respond" {
		return cmdMCPInputRespond(args[1:])
	}
	fs := newFlagSet("mcp inputs")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	rounds, err := client.ListMCPInputs(context.Background())
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(rounds)
	}
	if len(rounds) == 0 {
		fmt.Println("no external MCP server is waiting on an answer")
		return nil
	}
	for _, round := range rounds {
		fmt.Printf("%s  %s  (round %d of %d, sandbox %s)\n",
			round.ID, round.Tool, round.Round, round.RoundLimit, round.SandboxID)
		for _, q := range round.Questions {
			fmt.Printf("    [%s] %s\n", q.Kind, q.Message)
		}
		fmt.Printf("    caged mcp inputs respond %s --answer '<id>={\"key\":\"value\"}'\n", round.ID)
		fmt.Printf("    caged mcp inputs respond %s --decline\n\n", round.ID)
	}
	fmt.Println("These are questions third-party MCP servers asked mid-call. Caged routes them to a")
	fmt.Println("person rather than letting the agent's model answer on your behalf; the call")
	fmt.Println("completes when the agent retries it after you answer.")
	return nil
}

func cmdMCPInputRespond(args []string) error {
	fs := newFlagSet("mcp inputs respond")
	var (
		decline = fs.Bool("decline", false, "refuse the question; the server is told no, once")
		note    = fs.String("note", "", "a note recorded with the decision")
	)
	var answers []api.MCPInputAnswer
	fs.Func("answer", "one answer as <question-id>=<json> (repeatable)", func(v string) error {
		id, raw, ok := strings.Cut(v, "=")
		if !ok {
			return fmt.Errorf("--answer must be <question-id>=<json>, got %q", v)
		}
		if !json.Valid([]byte(raw)) {
			return fmt.Errorf("the answer for %q is not valid JSON", id)
		}
		answers = append(answers, api.MCPInputAnswer{ID: id, Content: json.RawMessage(raw)})
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged mcp inputs respond <id> --answer '<qid>={...}' | --decline")
	}
	if !*decline && len(answers) == 0 {
		return fmt.Errorf("--answer or --decline is required")
	}
	client, err := mustClient()
	if err != nil {
		return err
	}
	if err := client.RespondToMCPInput(context.Background(), fs.Arg(0), answers, *decline, *note); err != nil {
		return err
	}
	fmt.Println("recorded. The agent's next attempt at the same call carries this to the server.")
	fmt.Println("Caged does not re-send the call itself: a tool call whose side effect may be")
	fmt.Println("half-done must not be repeated by infrastructure.")
	return nil
}

// --- usage -------------------------------------------------------------

func printMCPUsage() {
	fmt.Printf(`caged mcp — third-party MCP servers, and the stdio bridge

Usage: caged mcp <command> [options]
       caged mcp <sandbox-id>            bridge stdio to a sandbox's MCP endpoint

Servers:
  catalogue                              the servers Caged has reviewed
  servers add --alias A --endpoint URL    register one
  servers list                            what is registered
  servers show <id>                       one server and its pinned tools
  servers refresh <id>                    re-fetch its catalogue; report what changed
  servers rm <id>                         deregister it

Visibility:
  bind <server-id> [--persona ID]         make it visible to a subject
    [--tools a,b] [--pinned] [--allow-tools]
  unbind <binding-id>                     remove a binding
  bindings                                what is bound to whom
  tools [server-id]                       the pinned catalogue, as an agent sees it

Policy — read this if calls are being refused:
  readiness --persona ID                  can this persona actually call them?
  allow <server-id> --persona ID          write the one rule that lets it
  disallow <server-id> --persona ID       remove the rule Caged wrote

Review:
  tools diff <server-id> <tool>           what changed in a quarantined definition
  tools approve <server-id> <tool>        release it
  tools reject <server-id> <tool>         refuse it, durably

OAuth:
  oauth show <server-id>                  what is authorized, and what it would involve
  oauth consent <server-id> --persona ID  record your decision (forwards nothing)
  oauth authorize <server-id>             get the URL to open

Questions from servers:
  inputs                                  what is waiting on a person
  inputs respond <id> --answer 'q={...}'   answer it
  inputs respond <id> --decline            refuse it

Two things that will otherwise cost you an afternoon:

  Binding a server does NOT make its tools callable. Caged's autonomy tiers rank
  its own tools, so a third-party tool is unclassified and denied at every tier —
  including autonomous. 'caged mcp bind --allow-tools' writes the rule; 'caged
  mcp readiness' says which servers still need it.

  A 'quarantined' tool is one whose definition CHANGED since a human approved it.
  It is advertised to no agent and fails the gate if called anyway. Read 'caged
  mcp tools diff' before approving: approving without reading is the outcome the
  mechanism exists to prevent.

`)
}

// --- helpers -----------------------------------------------------------

func printPolicyAdvice(advice *api.MCPPolicyAdvice, serverID, personaID string) {
	if advice == nil {
		return
	}
	fmt.Printf("\npolicy: %s (%d of %d approved tools allowed)\n",
		advice.Status, advice.ToolsAllowed, advice.ToolsEvaluated)
	fmt.Printf("%s\n", wrapText(advice.Explanation, 78))
	if advice.Remedy == "allow_mcp_server" && advice.GrantedRule == false {
		fmt.Printf("\n  caged mcp allow %s", serverID)
		if personaID != "" {
			fmt.Printf(" --persona %s", personaID)
		}
		fmt.Println()
	}
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitScopes accepts spaces or commas, because an operator copying a scope
// list out of a provider's docs gets spaces and one typing it gets commas.
func splitScopes(raw string) []string {
	replaced := strings.ReplaceAll(raw, ",", " ")
	return strings.Fields(replaced)
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func preview(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) > 3 {
		return "(" + strings.Join(names[:3], ", ") + ", …)"
	}
	return "(" + strings.Join(names, ", ") + ")"
}

func indentJSON(raw json.RawMessage) string {
	var buf strings.Builder
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("  ", "  ")
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "  " + string(raw)
	}
	if err := encoder.Encode(value); err != nil {
		return "  " + string(raw)
	}
	return "  " + strings.TrimRight(buf.String(), "\n")
}

// wrapText wraps at word boundaries, so a paragraph Caged wrote to be read is
// readable in a terminal rather than one long line.
func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var (
		lines []string
		line  strings.Builder
	)
	for _, word := range words {
		if line.Len() > 0 && line.Len()+1+len(word) > width {
			lines = append(lines, line.String())
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(word)
	}
	lines = append(lines, line.String())
	return strings.Join(lines, "\n")
}

// newFlagSet builds a flag set that reports usage errors rather than exiting.
//
// flag.ExitOnError — which the a2a commands use — calls os.Exit from inside the
// parser, so a caller cannot add a sentence of its own to a bad flag. Every
// refusal in this file names what it refused, and that requires the error back.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// printJSON writes a value as indented JSON, for --json.
func printJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding output: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// confirm asks a yes/no question on the terminal.
//
// It returns FALSE on any read error, including a closed stdin. A consent
// prompt that defaults to yes when it cannot read the answer is not a prompt,
// and this one gates a forward to a third party.
func confirm(question string) bool {
	fmt.Printf("%s [y/N] ", question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
