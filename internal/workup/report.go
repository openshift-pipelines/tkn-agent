package workup

import (
	"context"
	"encoding/json"
	"errors"
	"os"
)

type runtimeError struct {
	message string
	cause   error
}

func (e runtimeError) Error() string { return e.message }
func (e runtimeError) Unwrap() error { return e.cause }

func finishReport(report *Report, secrets []string, resultErr *error) {
	if *resultErr != nil {
		report.State = "incomplete"
		var cause error
		switch {
		case errors.Is(*resultErr, context.Canceled):
			report.State, cause = "cancelled", context.Canceled
		case errors.Is(*resultErr, context.DeadlineExceeded):
			report.State, cause = "cancelled", context.DeadlineExceeded
		}
		report.Reason = redactText((*resultErr).Error(), secrets)
		if len(report.Reason) > 4096 {
			report.Reason = report.Reason[:4096] + " [error truncated; evidence incomplete]"
		}
		*resultErr = runtimeError{message: report.Reason, cause: cause}
		report.MissingEvidence = append(report.MissingEvidence, report.Reason)
	}
	for _, run := range report.Runs {
		if run.Outcome == "failed" {
			report.State = "failed"
		}
	}
	report.Message = report.Reason
	if report.Message == "" {
		report.Message = "Diagnostic evidence collected; workload health requires interpretation."
	}
	data, err := json.Marshal(report)
	if err != nil {
		*resultErr = errors.Join(*resultErr, err)
		return
	}
	if err := json.Unmarshal(redactJSON(data, secrets), report); err != nil {
		*resultErr = errors.Join(*resultErr, err)
	}
}

func providerSecrets() []string {
	var values []string
	for _, name := range []string{"OPENAI_API_KEY", "GOOGLE_API_KEY", "GEMINI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		if value := os.Getenv(name); value != "" {
			values = append(values, value)
		}
	}
	return values
}
