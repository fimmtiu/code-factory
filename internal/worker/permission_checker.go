package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"

	acp "github.com/coder/acp-go-sdk"
)

// permissionCheckerBinary is the external program consulted to vet permission
// requests before falling back to an interactive user prompt. It is looked up
// on PATH; when it isn't installed, the checker is silently skipped and the
// existing prompt behaviour applies.
const permissionCheckerBinary = "claude-permissions-checker"

// checkerRequest is the JSON sent to claude-permissions-checker on stdin. It
// mirrors the subset of the ACP permission request the checker reads: the
// toolCallId carries the semantic operation string, which in the ACP flow is
// the tool-call title (e.g. "Read /path", "Edit `/path`", "Find `dir` `glob`",
// or a backtick-wrapped bash command).
type checkerRequest struct {
	SessionID string `json:"sessionId"`
	ToolCall  struct {
		ToolCallID string `json:"toolCallId"`
	} `json:"toolCall"`
}

// checkerDecision is the JSON claude-permissions-checker prints on stdout. Both
// allow and deny are reported with exit code 0; only genuine errors exit
// non-zero. Result is "allow" or "deny".
type checkerDecision struct {
	Result  string `json:"result"`
	Command string `json:"command,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// checkPermission consults the external claude-permissions-checker program to
// decide whether a permission request is safe to auto-approve. It returns
// approved=true only when the checker explicitly allows the request. Any other
// outcome — a denial, a missing binary, a malformed response, or an execution
// error — returns false so the caller falls back to prompting the user. On a
// denial the returned reason carries the checker's explanation for logging.
func checkPermission(ctx context.Context, workdir, sessionID, toolCallID string) (approved bool, reason string) {
	bin, err := exec.LookPath(permissionCheckerBinary)
	if err != nil {
		return false, "" // not installed; fall back to prompting
	}

	var req checkerRequest
	req.SessionID = sessionID
	req.ToolCall.ToolCallID = toolCallID
	input, err := json.Marshal(req)
	if err != nil {
		return false, ""
	}

	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = workdir
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		return false, "" // missing config, bad input, etc.; fall back to prompting
	}

	var decision checkerDecision
	if err := json.Unmarshal(out, &decision); err != nil {
		return false, ""
	}
	if decision.Result == "allow" {
		return true, ""
	}
	return false, decision.Reason
}

// selectAllowAlways picks the broadest "allow" option from the offered set,
// preferring an always-allow grant so an approved request won't prompt again
// for equivalent calls. It falls back to allow-once, then to the first option.
// The bool is false only when no options were offered.
func selectAllowAlways(options []acp.PermissionOption) (acp.RequestPermissionResponse, bool) {
	for _, want := range []acp.PermissionOptionKind{
		acp.PermissionOptionKindAllowAlways,
		acp.PermissionOptionKindAllowOnce,
	} {
		for _, o := range options {
			if o.Kind == want {
				return acp.RequestPermissionResponse{
					Outcome: acp.NewRequestPermissionOutcomeSelected(o.OptionId),
				}, true
			}
		}
	}
	if len(options) > 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeSelected(options[0].OptionId),
		}, true
	}
	return acp.RequestPermissionResponse{}, false
}
