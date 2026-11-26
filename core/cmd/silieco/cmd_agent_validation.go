package main

import (
	"fmt"

	"github.com/auenger/MEPERECO/core/internal/agentconfig"
)

func validateAgentMaxConcurrentRunsFlag(value int32) error {
	if err := agentconfig.ValidateMaxConcurrentRuns(value); err != nil {
		return fmt.Errorf(
			"--max-concurrent-runs %w (got %d)",
			err,
			value,
		)
	}
	return nil
}
