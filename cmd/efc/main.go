// Command efc is the Elastic Fruit Cloud command line for people and AI
// agents. It reads the API token from EFC_TOKEN and the API URL from
// EFC_API_URL. "efc mcp" serves the same actions as an MCP server over stdio.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/boring-design/elastic-fruit-cli/agenttools"
)

const (
	tokenEnv  = "EFC_TOKEN"
	apiURLEnv = "EFC_API_URL"

	outputText = "text"
	outputJSON = "json"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := newRootCommand().ExecuteContext(ctx)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "efc: "+agenttools.ErrorText(err))
		os.Exit(1)
	}
}

// options are the global flags.
type options struct {
	apiURL string
	output string
}

func newRootCommand() *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:           "efc",
		Short:         "Elastic Fruit Cloud command line and MCP server",
		Long:          "efc sets up Elastic Fruit Cloud and reads its jobs. It reads the API token from " + tokenEnv + " and the API URL from " + apiURLEnv + ".",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if opts.output != outputText && opts.output != outputJSON {
				return fmt.Errorf("--output is %q, use %s or %s", opts.output, outputText, outputJSON)
			}
			return nil
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(&opts.apiURL, "api-url", defaultAPIURL(), "Elastic Fruit Cloud API URL, default from "+apiURLEnv)
	root.PersistentFlags().StringVarP(&opts.output, "output", "o", outputText, "output format: text or json")
	root.AddCommand(
		newStatusCommand(opts),
		newInstallLinkCommand(opts),
		newReposCommand(opts),
		newJobsCommand(opts),
		newUsageCommand(opts),
		newMCPCommand(opts),
		newVersionCommand(opts),
	)
	return root
}

func defaultAPIURL() string {
	if value := os.Getenv(apiURLEnv); value != "" {
		return value
	}
	return agenttools.DefaultAPIURL
}

// tools returns the agent actions with the token from EFC_TOKEN.
func (o *options) tools() (*agenttools.Tools, error) {
	token := strings.TrimSpace(os.Getenv(tokenEnv))
	if token == "" {
		return nil, errors.New(tokenEnv + " is not set: sign in at " + o.apiURL + ", create an API token and export it as " + tokenEnv)
	}
	return agenttools.New(newAPIClient(o.apiURL, token)), nil
}

// print writes value as JSON with --output json, otherwise it calls text.
func (o *options) print(value any, text func(w io.Writer)) error {
	if o.output == outputJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	text(os.Stdout)
	return nil
}
