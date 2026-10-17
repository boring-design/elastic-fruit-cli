package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/boring-design/elastic-fruit-cli/agenttools"
)

func newReposCommand(opts *options) *cobra.Command {
	repos := &cobra.Command{
		Use:   "repos",
		Short: "List, enable and disable repositories",
	}
	repos.AddCommand(newReposAvailableCommand(opts), newReposListCommand(opts), newReposEnableCommand(opts), newReposDisableCommand(opts))
	return repos
}

func newReposAvailableCommand(opts *options) *cobra.Command {
	var account string
	command := &cobra.Command{
		Use:   "available",
		Short: "List the repositories the GitHub App installations can see",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			list, err := tools.AvailableRepositories(cmd.Context(), account)
			if err != nil {
				return err
			}
			return opts.print(list, func(w io.Writer) {
				if len(list.Repositories) == 0 {
					fmt.Fprintln(w, "No repositories are visible to the GitHub App installations of this tenant.")
					return
				}
				table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(table, "NAME\tPRIVATE\tARCHIVED\tENABLED")
				for _, repository := range list.Repositories {
					fmt.Fprintf(table, "%s\t%t\t%t\t%t\n", repository.FullName, repository.Private, repository.Archived, repository.Enabled)
				}
				table.Flush()
			})
		},
	}
	command.Flags().StringVar(&account, "account", "", "only repositories of this GitHub account")
	return command
}

func newReposListCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the enabled repositories with their status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			list, err := tools.Repositories(cmd.Context())
			if err != nil {
				return err
			}
			return opts.print(list, func(w io.Writer) { printRepositories(w, list.Repositories) })
		},
	}
}

func newReposEnableCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <owner/name>",
		Short: "Enable a repository, needs an admin token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			enabled, err := tools.EnableRepository(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return opts.print(enabled, func(w io.Writer) {
				fmt.Fprintf(w, "Enabled %s, status %s.\n%s\n", enabled.Repository.FullName, enabled.Repository.Status, enabled.Message)
			})
		},
	}
}

func newReposDisableCommand(opts *options) *cobra.Command {
	var confirmed bool
	command := &cobra.Command{
		Use:   "disable <owner/name>",
		Short: "Disable a repository, needs an admin token and --yes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirmed {
				return fmt.Errorf("disabling %s stops its jobs from running on Elastic Fruit Cloud, run again with --yes", args[0])
			}
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			disabled, err := tools.DisableRepository(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return opts.print(disabled, func(w io.Writer) {
				fmt.Fprintf(w, "Disabled %s. Its job history stays.\n", disabled.FullName)
			})
		},
	}
	command.Flags().BoolVar(&confirmed, "yes", false, "confirm disabling the repository")
	return command
}

func printRepositories(w io.Writer, repositories []agenttools.Repository) {
	if len(repositories) == 0 {
		fmt.Fprintln(w, "  none")
		return
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "  NAME\tSTATUS\tJOBS 7 DAYS\tLAST JOB")
	for _, repository := range repositories {
		lastJob := "none"
		if repository.LastJob != nil {
			lastJob = repository.LastJob.Result + " " + repository.LastJob.StartedAt
		}
		fmt.Fprintf(table, "  %s\t%s\t%d\t%s\n", repository.FullName, repository.Status, repository.JobsLast7Days, lastJob)
		if repository.Problem != "" {
			fmt.Fprintf(table, "  \t%s\t\t\n", repository.Problem)
		}
	}
	table.Flush()
}
