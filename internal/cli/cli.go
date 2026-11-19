// Package cli exposes pipeline-aware local verification to humans and hooks.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// IO separates the structured result stream from diagnostics.
type IO struct {
	Out io.Writer
	Err io.Writer
}

type options struct {
	root, pipeline, provider, model, approve, format string
	timeout                                          time.Duration
	local, refresh                                   bool
	checkout                                         string
	parameters                                       []string
}

// Run implements the process exit contract shared by direct and hook invocation.
func Run(ctx context.Context, args []string, streams IO) int {
	opts := options{}
	code := 0
	root := &cobra.Command{Use: "tkn-agent", SilenceUsage: true, SilenceErrors: true}
	verify := &cobra.Command{
		Use: "verify --local PIPELINE", Short: "Select and verify a reviewed local subset of CI",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.pipeline = args[0]
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("format must be text or json")
			}
			result, exit := verification(cmd.Context(), opts)
			code = exit
			if opts.format == "json" {
				return json.NewEncoder(streams.Out).Encode(result)
			}
			if _, err := fmt.Fprintf(streams.Out, "%s: %s\n%d selected, %d provided, %d skipped, %d unresolved; cluster CI remains required\n",
				result.State, result.Message, result.Selected, result.Provided, result.Skipped, result.Unresolved); err != nil {
				return err
			}
			for _, check := range result.Checks {
				if _, err := fmt.Fprintf(streams.Out, "%s: %s (exit %d)\n%s", check.ID, check.Outcome, check.ExitCode, check.Output); err != nil {
					return err
				}
			}
			if result.State == "review-required" {
				if _, err := fmt.Fprintf(streams.Out, "Review this plan, then use --approve-plan %s\n", result.Digest); err != nil {
					return err
				}
				return json.NewEncoder(streams.Out).Encode(result.Plan)
			}
			return nil
		},
	}
	flags := verify.Flags()
	flags.BoolVar(&opts.local, "local", false, "Use existing local Podman")
	flags.StringVar(&opts.root, "repo", ".", "Repository root")
	flags.StringVar(&opts.provider, "provider", os.Getenv("TKN_AGENT_PROVIDER"), "Planner provider: openai, gemini, anthropic")
	flags.StringVar(&opts.model, "model", os.Getenv("TKN_AGENT_MODEL"), "Explicit provider model name")
	flags.StringVar(&opts.approve, "approve-plan", "", "Accept exactly the displayed plan digest")
	flags.StringVar(&opts.format, "format", "text", "Result format: text or json")
	flags.DurationVar(&opts.timeout, "timeout", 3*time.Minute, "Deadline per selected check, including startup")
	flags.BoolVar(&opts.refresh, "refresh", false, "Analyze again instead of reusing the plan")
	flags.StringVar(&opts.checkout, "bind-checkout", "", "Explicitly provide TASK's reviewed self-checkout from the current worktree")
	flags.StringArrayVar(&opts.parameters, "param", nil, "Bind a declared string parameter NAME=VALUE (repeatable)")
	root.AddCommand(verify)
	root.AddCommand(workupCommand(streams, &code))
	root.SetArgs(args)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	if err := root.ExecuteContext(ctx); err != nil {
		if _, writeErr := fmt.Fprintln(streams.Err, err); writeErr != nil {
			return 2
		}
		return 2
	}
	return code
}
