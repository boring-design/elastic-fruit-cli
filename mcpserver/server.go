// Package mcpserver offers the agent tools of package agenttools as an MCP
// server, over stdio for the efc CLI and over stateless Streamable HTTP at
// /mcp of the cloud server.
package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/boring-design/elastic-fruit-cli/agenttools"
)

// ServerName is the name the MCP server reports to clients.
const ServerName = "efc"

// schemaCache keeps the JSON schemas of the tools, remote MCP builds a server per request.
var schemaCache = mcp.NewSchemaCache()

const serverInstructions = "Elastic Fruit Cloud runs GitHub Actions jobs that use `runs-on: elastic-fruit`. " +
	"Start with efc_get_status and follow next_step. " +
	"Installing the GitHub App is done by a person: give them the link from efc_get_install_link. " +
	"Report API errors as they are, with their code and message."

type noInput struct{}

type availableRepositoriesInput struct {
	Account string `json:"account,omitempty" jsonschema:"keep only repositories of this GitHub account"`
}

type repositoryInput struct {
	Repository string `json:"repository" jsonschema:"the repository as owner/name"`
}

type jobsInput struct {
	Repository    string `json:"repository,omitempty" jsonschema:"keep only jobs of this enabled repository, as owner/name"`
	WorkflowRunID int64  `json:"workflow_run_id,omitempty" jsonschema:"keep only jobs of this GitHub workflow run id"`
	Result        string `json:"result,omitempty" jsonschema:"keep only jobs with this result: succeeded, failed, canceled, lost or running"`
	Page          int    `json:"page,omitempty" jsonschema:"page of 50 jobs starting at 1, used only without workflow_run_id and result"`
}

type jobInput struct {
	JobID string `json:"job_id" jsonschema:"the Elastic Fruit Cloud job id"`
}

type jobLogsInput struct {
	JobID    string `json:"job_id" jsonschema:"the Elastic Fruit Cloud job id"`
	MaxBytes int32  `json:"max_bytes,omitempty" jsonschema:"how many bytes of the end of the log to return, default 32768, at most 1048576"`
}

type usageInput struct {
	Days int32 `json:"days,omitempty" jsonschema:"number of days back from today, today included, 1 to 366, default 30"`
}

// NewServer returns an MCP server with the agent tools. It offers no tool
// that creates enrollment tokens, manages members, deletes anything other
// than the enabled state of a repository, or links installations.
func NewServer(tools *agenttools.Tools) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: ServerName, Version: agenttools.Version()},
		&mcp.ServerOptions{Instructions: serverInstructions, SchemaCache: schemaCache},
	)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	notDestructive := false
	destructive := true

	addTool(server, &mcp.Tool{
		Name:        "efc_get_status",
		Description: "Show the setup state of the tenant of the token: GitHub App installations, enabled repositories, job counts, quota and the next step.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ noInput) (agenttools.Status, error) {
		return tools.Status(ctx)
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_get_install_link",
		Description: "Get the GitHub App install link for a person to click. It works for 15 minutes. This tool does not install or link anything.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ noInput) (agenttools.InstallLink, error) {
		return tools.InstallLink(ctx)
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_list_available_repositories",
		Description: "List the repositories the GitHub App installations of the tenant can see, and whether each is enabled.",
		Annotations: readOnly,
	}, func(ctx context.Context, in availableRepositoriesInput) (agenttools.AvailableRepositoryList, error) {
		return tools.AvailableRepositories(ctx, in.Account)
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_enable_repository",
		Description: "Enable a repository so its workflow jobs with `runs-on: elastic-fruit` run on Elastic Fruit Cloud. Needs an admin token.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &notDestructive, IdempotentHint: true},
	}, func(ctx context.Context, in repositoryInput) (agenttools.EnabledRepository, error) {
		return tools.EnableRepository(ctx, in.Repository)
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_disable_repository",
		Description: "Disable an enabled repository: its jobs stop running on Elastic Fruit Cloud, job history stays. Needs an admin token.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, IdempotentHint: true},
	}, func(ctx context.Context, in repositoryInput) (agenttools.DisabledRepository, error) {
		return tools.DisableRepository(ctx, in.Repository)
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_list_repositories",
		Description: "List the enabled repositories of the tenant with their status, problem, last job and job count of the last 7 days.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ noInput) (agenttools.RepositoryList, error) {
		return tools.Repositories(ctx)
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_list_jobs",
		Description: "List jobs of the tenant, newest first, optionally of one repository, one workflow run or one result.",
		Annotations: readOnly,
	}, func(ctx context.Context, in jobsInput) (agenttools.JobList, error) {
		return tools.Jobs(ctx, agenttools.JobFilter{Repository: in.Repository, WorkflowRunID: in.WorkflowRunID, Result: in.Result, Page: in.Page})
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_get_job",
		Description: "Show one job with a summary of its resource samples and where its log is.",
		Annotations: readOnly,
	}, func(ctx context.Context, in jobInput) (agenttools.JobDetail, error) {
		return tools.Job(ctx, in.JobID)
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_get_job_logs",
		Description: "Read the end of the GitHub log of one job. When the log cannot be read, status says why and the result links the run on GitHub with a gh command.",
		Annotations: readOnly,
	}, func(ctx context.Context, in jobLogsInput) (agenttools.JobLogs, error) {
		return tools.JobLogs(ctx, in.JobID, in.MaxBytes)
	})
	addTool(server, &mcp.Tool{
		Name:        "efc_get_usage",
		Description: "Show the daily usage of the tenant per repository for the last days, with totals.",
		Annotations: readOnly,
	}, func(ctx context.Context, in usageInput) (agenttools.Usage, error) {
		return tools.Usage(ctx, in.Days)
	})
	return server
}

// addTool registers one tool. An error becomes a tool error with the text of
// agenttools.ErrorText, so the agent sees the API error code and message.
func addTool[In, Out any](server *mcp.Server, tool *mcp.Tool, run func(context.Context, In) (Out, error)) {
	mcp.AddTool(server, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		out, err := run(ctx, in)
		if err != nil {
			var empty Out
			return nil, empty, errors.New(agenttools.ErrorText(err))
		}
		return nil, out, nil
	})
}
