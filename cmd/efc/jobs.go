package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/boring-design/elastic-fruit-cli/agenttools"
)

func newJobsCommand(opts *options) *cobra.Command {
	jobs := &cobra.Command{
		Use:   "jobs",
		Short: "List jobs, show one job and read its log",
	}
	jobs.AddCommand(newJobsListCommand(opts), newJobsGetCommand(opts), newJobsLogsCommand(opts))
	return jobs
}

func newJobsListCommand(opts *options) *cobra.Command {
	var filter agenttools.JobFilter
	command := &cobra.Command{
		Use:   "list",
		Short: "List jobs, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			list, err := tools.Jobs(cmd.Context(), filter)
			if err != nil {
				return err
			}
			return opts.print(list, func(w io.Writer) { printJobs(w, list) })
		},
	}
	command.Flags().StringVar(&filter.Repository, "repo", "", "only jobs of this enabled repository, owner/name")
	command.Flags().Int64Var(&filter.WorkflowRunID, "run-id", 0, "only jobs of this GitHub workflow run id")
	command.Flags().StringVar(&filter.Result, "result", "", "only jobs with this result: succeeded, failed, canceled, lost or running")
	command.Flags().IntVar(&filter.Page, "page", 1, "page of 50 jobs, used only without --run-id and --result")
	return command
}

func printJobs(w io.Writer, list agenttools.JobList) {
	if len(list.Jobs) > 0 {
		table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "ID\tREPOSITORY\tJOB\tRESULT\tSTARTED\tRUN")
		for _, job := range list.Jobs {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%d\n", job.ID, job.Repository, job.DisplayName, job.Result, job.StartedAt, job.WorkflowRunID)
		}
		table.Flush()
	} else if list.Message == "" {
		fmt.Fprintln(w, "No jobs match.")
	}
	if list.Message != "" {
		fmt.Fprintln(w, list.Message)
	}
	if list.HasMore {
		fmt.Fprintln(w, "More jobs exist.")
	}
}

func newJobsGetCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "get <job-id>",
		Short: "Show one job with a summary of its resource samples",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			detail, err := tools.Job(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return opts.print(detail, func(w io.Writer) { printJob(w, detail) })
		},
	}
}

func printJob(w io.Writer, detail agenttools.JobDetail) {
	job := detail.Job
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(table, "Job\t%s\n", job.ID)
	fmt.Fprintf(table, "Repository\t%s\n", job.Repository)
	fmt.Fprintf(table, "Name\t%s\n", job.DisplayName)
	fmt.Fprintf(table, "Workflow\t%s\n", job.WorkflowRef)
	fmt.Fprintf(table, "Event\t%s\n", job.EventName)
	fmt.Fprintf(table, "Result\t%s\n", job.Result)
	fmt.Fprintf(table, "Queued\t%s\n", job.QueuedAt)
	fmt.Fprintf(table, "Started\t%s\n", job.StartedAt)
	fmt.Fprintf(table, "Finished\t%s\n", job.FinishedAt)
	fmt.Fprintf(table, "Runner\t%s on %s\n", job.RunnerName, job.AgentName)
	if detail.RunnerEndReason != "" {
		fmt.Fprintf(table, "Runner end\t%s\n", detail.RunnerEndReason)
	}
	fmt.Fprintf(table, "Samples\t%d, max CPU %.1f%%, max memory %d bytes\n", detail.Samples.Count, detail.Samples.MaxCPUPercent, detail.Samples.MaxMemoryUsedBytes)
	fmt.Fprintf(table, "Run on GitHub\t%s\n", job.GitHubRunURL)
	table.Flush()
	fmt.Fprintln(w)
	fmt.Fprintln(w, detail.LogsHint.Text)
	if detail.LogsHint.Command != "" {
		fmt.Fprintln(w, "  "+detail.LogsHint.Command)
	}
}

func newJobsLogsCommand(opts *options) *cobra.Command {
	var maxBytes int32
	command := &cobra.Command{
		Use:   "logs <job-id>",
		Short: "Print the end of the GitHub log of one job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			logs, err := tools.JobLogs(cmd.Context(), args[0], maxBytes)
			if err != nil {
				return err
			}
			return opts.print(logs, func(w io.Writer) { printJobLogs(w, logs) })
		},
	}
	command.Flags().Int32Var(&maxBytes, "max-bytes", 0, "how many bytes of the end of the log to print, default 32768, at most 1048576")
	return command
}

// printJobLogs writes a ready log to w and notes about it to stderr, so the
// log can be piped. Any other status is written to w with where the log is.
func printJobLogs(w io.Writer, logs agenttools.JobLogs) {
	if logs.Status == "ready" {
		fmt.Fprint(w, logs.Log)
		if logs.Message != "" {
			fmt.Fprintln(os.Stderr, logs.Message)
		}
		return
	}
	fmt.Fprintf(w, "Status: %s\n%s\n", logs.Status, logs.Message)
	if logs.GitHubRunURL != "" {
		fmt.Fprintf(w, "Run on GitHub: %s\n", logs.GitHubRunURL)
	}
	if logs.LogCommand != "" {
		fmt.Fprintf(w, "Read the log with: %s\n", logs.LogCommand)
	}
}
