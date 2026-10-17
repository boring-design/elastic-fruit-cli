package main

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/boring-design/elastic-fruit-cli/mcpserver"
)

func newMCPCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve the efc tools as an MCP server over stdio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tools, err := opts.tools()
			if err != nil {
				return err
			}
			return mcpserver.NewServer(tools).Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
}
