# elastic-fruit-cli

`efc` is the command line and MCP server of [Elastic Fruit Cloud](https://elastic-fruit.strrl.cloud). People and AI agents use it to set up Elastic Fruit Cloud for their GitHub repositories and to read jobs, job logs and usage. Workflow jobs run on Elastic Fruit Cloud with `runs-on: elastic-fruit`.

It talks to the public Cloud API, `cloud.v1.CloudService` from [elastic-fruit-protocol](https://github.com/boring-design/elastic-fruit-protocol).

## Install

Download the binary for your system from [GitHub Releases](https://github.com/boring-design/elastic-fruit-cli/releases), or build it with Go:

```sh
go install github.com/boring-design/elastic-fruit-cli/cmd/efc@latest
```

## Sign in and token

1. Sign in at https://elastic-fruit.strrl.cloud with GitHub.
2. Create a tenant API token on the API tokens page. A token belongs to one tenant and has the scope `read` or `admin`. Enabling and disabling repositories needs `admin`.
3. Export it:

```sh
export EFC_TOKEN=efc_...
```

| Variable | Description |
| --- | --- |
| `EFC_TOKEN` | Tenant API token, `efc_` followed by 43 base64url characters. Required. |
| `EFC_API_URL` | API URL, default `https://elastic-fruit.strrl.cloud`. |

## Commands

| Command | What it does | Scope |
| --- | --- | --- |
| `efc status` | Installations, enabled repositories, quota and the next step | read |
| `efc install-link` | GitHub App install link for a person to click | read |
| `efc repos available [--account X]` | Repositories the GitHub App can see | read |
| `efc repos list` | Enabled repositories with their status | read |
| `efc repos enable <owner/name>` | Enable a repository | admin |
| `efc repos disable <owner/name> --yes` | Disable a repository, job history stays | admin |
| `efc jobs list [--repo R] [--run-id N] [--result failed] [--page N]` | Jobs, newest first | read |
| `efc jobs get <job-id>` | One job with a summary of its resource samples | read |
| `efc jobs logs <job-id> [--max-bytes N]` | End of the GitHub log of one job, default 32768 bytes, at most 1048576 | read |
| `efc usage [--days N]` | Daily usage per repository | read |
| `efc mcp` | MCP server over stdio with the tools below | token scope |
| `efc version` | Version of efc and of the server | none |

Every command takes `--output json` for scripts and `--api-url` to override `EFC_API_URL`.

## MCP

The MCP server is named `efc`. Use the remote server:

```sh
claude mcp add --transport http efc https://elastic-fruit.strrl.cloud/mcp --header "Authorization: Bearer $EFC_TOKEN"
```

or run it locally over stdio with `efc mcp`. The remote server accepts only the bearer token, never a browser cookie.

Tools: `efc_get_status`, `efc_get_install_link`, `efc_list_available_repositories`, `efc_enable_repository`, `efc_disable_repository`, `efc_list_repositories`, `efc_list_jobs`, `efc_get_job`, `efc_get_job_logs`, `efc_get_usage`.

The tools do not create enrollment tokens, manage members or delete anything except the enabled state of a repository. API errors are reported as `EFC API error <code>: <message>`, as the server sent them.

## Steps only a person can do

- Sign in with GitHub and create the API token.
- Install the GitHub App: the tools only return the install link. The link works for 15 minutes. The person who clicks it must be signed in to Elastic Fruit Cloud as a member of the tenant first, and must install the App on GitHub themselves.

## Job logs

`efc jobs logs` and `efc_get_job_logs` read the end of a job log from GitHub. The GitHub App needs the Actions read permission for this. When it cannot read the log, the status says why: `log_not_ready`, `permission_not_approved`, `repository_not_accessible` or `job_not_located`. Every answer links the run on GitHub, and `gh run view <workflow_run_id> --repo <owner>/<repo> --log` reads the log with the GitHub CLI.

## Go packages

- `agenttools` holds the actions, one method per tool, on top of a `cloudv1connect.CloudServiceClient`.
- `mcpserver` serves them over MCP: `NewServer` for stdio, `NewHTTPHandler` for stateless Streamable HTTP in front of a CloudService handler in the same process.

## License

Apache-2.0, see `LICENSE`.
