package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/boring-design/elastic-fruit-cli/agenttools"
)

func newUsageCommand(opts *options) *cobra.Command {
	var days int32
	command := &cobra.Command{
		Use:   "usage",
		Short: "Show the daily usage per repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			usage, err := tools.Usage(cmd.Context(), days)
			if err != nil {
				return err
			}
			return opts.print(usage, func(w io.Writer) { printUsage(w, usage) })
		},
	}
	command.Flags().Int32Var(&days, "days", 30, "number of days back from today, today included, 1 to 366")
	return command
}

func printUsage(w io.Writer, usage agenttools.Usage) {
	fmt.Fprintf(w, "Usage from %s to %s (UTC days)\n", usage.From, usage.To)
	totals := usage.Totals
	fmt.Fprintf(w, "Total: %d jobs (%d succeeded, %d failed, %d other), job time %ds, runner time %ds, pool runner time %ds\n\n",
		totals.JobCount, totals.SucceededCount, totals.FailedCount, totals.OtherCount, totals.JobSeconds, totals.RunnerSeconds, totals.PoolRunnerSeconds)
	if len(usage.Rows) == 0 {
		fmt.Fprintln(w, "No usage in these days.")
		return
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "DAY\tREPOSITORY\tJOBS\tFAILED\tJOB SECONDS\tRUNNER SECONDS\tPOOL SECONDS")
	for _, row := range usage.Rows {
		fmt.Fprintf(table, "%s\t%s\t%d\t%d\t%d\t%d\t%d\n", row.Day, row.Repository, row.JobCount, row.FailedCount, row.JobSeconds, row.RunnerSeconds, row.PoolRunnerSeconds)
	}
	table.Flush()
}
