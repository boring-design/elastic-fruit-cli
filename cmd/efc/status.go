package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/boring-design/elastic-fruit-cli/agenttools"
)

func newStatusCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show installations, enabled repositories, quota and the next step",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			status, err := tools.Status(cmd.Context())
			if err != nil {
				return err
			}
			return opts.print(status, func(w io.Writer) { printStatus(w, status) })
		},
	}
}

func printStatus(w io.Writer, status agenttools.Status) {
	fmt.Fprintf(w, "Server:    %s (version %s)\n", status.PublicURL, status.Version)
	fmt.Fprintf(w, "Next step: %s\n           %s\n\n", status.NextStep.Code, status.NextStep.Message)

	fmt.Fprintln(w, "Installations:")
	if len(status.Installations) == 0 {
		fmt.Fprintln(w, "  none")
	} else {
		table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "  ACCOUNT\tTYPE\tREPOSITORIES\tSUSPENDED")
		for _, installation := range status.Installations {
			fmt.Fprintf(table, "  %s\t%s\t%s\t%t\n", installation.AccountLogin, installation.AccountType, installation.RepositorySelection, installation.Suspended)
		}
		table.Flush()
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Enabled repositories:")
	printRepositories(w, status.EnabledRepositories)
	fmt.Fprintln(w)

	quota := status.Quota
	fmt.Fprintf(w, "Jobs:  %d running, %d in the last 7 days\n", status.JobsRunning, status.JobsLast7Days)
	fmt.Fprintf(w, "Quota: repositories %d/%d, runners %d/%d, pool minutes this month %d/%d, pool runners %d/%d\n",
		quota.RepositoriesEnabled, quota.MaxRepositories,
		quota.RunnersAllocated, quota.MaxConcurrentRunners,
		quota.PoolMinutesUsed, quota.PoolMinutesMonthly,
		quota.PoolRunnersAllocated, quota.MaxPoolRunners)
}

func newInstallLinkCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "install-link",
		Short: "Print the GitHub App install link for a person to click",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			link, err := tools.InstallLink(cmd.Context())
			if err != nil {
				return err
			}
			return opts.print(link, func(w io.Writer) {
				fmt.Fprintln(w, link.URL)
				fmt.Fprintln(w)
				for _, line := range link.Instructions {
					fmt.Fprintln(w, "- "+line)
				}
			})
		},
	}
}

// versionInfo is the version of this binary and of the server.
type versionInfo struct {
	CLIVersion    string `json:"cli_version"`
	ServerVersion string `json:"server_version"`
	PublicURL     string `json:"public_url"`
}

func newVersionCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of efc and of the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, err := agenttools.New(newAPIClient(opts.apiURL, "")).ServiceInfo(cmd.Context())
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "efc %s\n", agenttools.Version())
				return fmt.Errorf("read the server version from %s: %w", opts.apiURL, err)
			}
			version := versionInfo{CLIVersion: agenttools.Version(), ServerVersion: info.Version, PublicURL: info.PublicURL}
			return opts.print(version, func(w io.Writer) {
				fmt.Fprintf(w, "efc %s\nserver %s at %s\n", version.CLIVersion, version.ServerVersion, version.PublicURL)
			})
		},
	}
}
