package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/openshift-pipelines/tkn-agent/internal/workup"
	"github.com/spf13/cobra"
)

func workupCommand(streams IO, exitCode *int) *cobra.Command {
	opts := workup.Options{}
	provider := os.Getenv("TKN_AGENT_PROVIDER")
	if provider == "" {
		provider = "openai"
	}
	format := "text"
	cmd := &cobra.Command{
		Use:   "workup GOAL",
		Short: "Select and run permitted diagnostic Pipelines using skills and MCP evidence",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("format must be text or json")
			}
			opts.Goal = args[0]
			report, runErr := workup.Run(cmd.Context(), opts)
			if runErr != nil || !opts.Execute || report.State != "completed" {
				*exitCode = 2
			}
			if format == "json" {
				if err := json.NewEncoder(streams.Out).Encode(report); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(streams.Out, "%s: %s\n", report.State, report.Message); err != nil {
					return err
				}
				if err := json.NewEncoder(streams.Out).Encode(report); err != nil {
					return err
				}
			}
			return runErr
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.ConfigPath, "config", "", "Reviewed workup config with selected skills, MCP tools and permitted Pipelines")
	flags.StringVar(&opts.Kubeconfig, "kubeconfig", "", "Explicit kubeconfig file; ambient cluster access is disabled")
	flags.StringVar(&opts.Context, "context", "", "Explicit kubeconfig context")
	flags.StringVarP(&opts.Namespace, "namespace", "n", "", "Namespace containing the permitted diagnostic Pipelines")
	flags.StringVar(&opts.Provider, "provider", provider, "Provider: openai, gemini, anthropic")
	flags.StringVar(&opts.Model, "model", os.Getenv("TKN_AGENT_MODEL"), "Explicit provider model name")
	flags.BoolVar(&opts.Execute, "execute", false, "Authorize execution within the reviewed config policy; otherwise report a proposed plan")
	flags.StringVar(&format, "format", "text", "Report format: text or json")
	flags.DurationVar(&opts.Timeout, "timeout", 10*time.Minute, "Overall workup deadline, including model, MCP and Pipeline execution")
	return cmd
}
