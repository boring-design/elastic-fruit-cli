package agenttools

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	cloudv1 "github.com/boring-design/elastic-fruit-protocol/gen/cloud/v1"
)

// installLinkLifetimeSeconds is how long an install link works, the install
// state the server signs into it expires after 15 minutes.
const installLinkLifetimeSeconds = 900

// Next step codes of Status, checked in this order.
const (
	NextStepInstallApp         = "install_app"
	NextStepEnableRepository   = "enable_repository"
	NextStepCheckRepository    = "check_repository"
	NextStepNoJobsObservedYet  = "no_jobs_observed_yet"
	NextStepReady              = "ready"
	noJobsObservedYetMessage   = "No jobs observed yet. Jobs run on EFC when a workflow uses `runs-on: elastic-fruit`."
	repositoryStatusProblem    = "problem"
	repositoryStatusPoolMinute = "pool_minutes_used_up"
)

// ServiceInfo is the version and public URL of the server.
type ServiceInfo struct {
	Version   string `json:"version"`
	PublicURL string `json:"public_url"`
}

// Status is the setup state of the tenant of the token.
type Status struct {
	PublicURL           string         `json:"public_url"`
	Version             string         `json:"version"`
	Quota               Quota          `json:"quota"`
	Installations       []Installation `json:"installations"`
	EnabledRepositories []Repository   `json:"enabled_repositories"`
	JobsRunning         int32          `json:"jobs_running"`
	JobsLast7Days       int32          `json:"jobs_last_7_days"`
	HasJobs             bool           `json:"has_jobs"`
	NextStep            NextStep       `json:"next_step"`
}

// NextStep says what to do next, based only on what the server reports.
type NextStep struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Quota is the free tier limits of the tenant next to the current use.
type Quota struct {
	MaxRepositories      int32 `json:"max_repositories"`
	RepositoriesEnabled  int32 `json:"repositories_enabled"`
	MaxConcurrentRunners int32 `json:"max_concurrent_runners"`
	RunnersAllocated     int32 `json:"runners_allocated"`
	PoolMinutesMonthly   int32 `json:"pool_minutes_monthly"`
	PoolMinutesUsed      int32 `json:"pool_minutes_used"`
	MaxPoolRunners       int32 `json:"max_pool_runners"`
	PoolRunnersAllocated int32 `json:"pool_runners_allocated"`
	MaxAgents            int32 `json:"max_agents"`
	AgentsUsed           int32 `json:"agents_used"`
}

// Installation is one GitHub account the GitHub App is installed on for the tenant.
type Installation struct {
	InstallationID      int64  `json:"installation_id"`
	AccountLogin        string `json:"account_login"`
	AccountType         string `json:"account_type"`
	RepositorySelection string `json:"repository_selection"`
	Suspended           bool   `json:"suspended"`
}

// InstallLink is the GitHub page where a person installs the GitHub App for the tenant.
type InstallLink struct {
	URL              string   `json:"url"`
	ExpiresInSeconds int      `json:"expires_in_seconds"`
	Instructions     []string `json:"instructions"`
}

// ServiceInfo reads the version and public URL of the server. It needs no token.
func (t *Tools) ServiceInfo(ctx context.Context) (ServiceInfo, error) {
	info, err := t.client.GetServiceInfo(ctx, connect.NewRequest(&cloudv1.GetServiceInfoRequest{}))
	if err != nil {
		return ServiceInfo{}, err
	}
	return ServiceInfo{Version: info.Msg.GetVersion(), PublicURL: info.Msg.GetPublicUrl()}, nil
}

// Status reads the installations, enabled repositories, counts and quota of the tenant.
func (t *Tools) Status(ctx context.Context) (Status, error) {
	info, err := t.ServiceInfo(ctx)
	if err != nil {
		return Status{}, err
	}
	overview, err := t.client.GetOverview(ctx, connect.NewRequest(&cloudv1.GetOverviewRequest{}))
	if err != nil {
		return Status{}, err
	}
	installations, err := t.client.ListInstallations(ctx, connect.NewRequest(&cloudv1.ListInstallationsRequest{}))
	if err != nil {
		return Status{}, err
	}
	repositories, err := t.client.ListRepositories(ctx, connect.NewRequest(&cloudv1.ListRepositoriesRequest{}))
	if err != nil {
		return Status{}, err
	}
	status := Status{
		PublicURL:           info.PublicURL,
		Version:             info.Version,
		Quota:               quotaFromProto(overview.Msg.GetQuota()),
		Installations:       make([]Installation, 0, len(installations.Msg.GetInstallations())),
		EnabledRepositories: make([]Repository, 0, len(repositories.Msg.GetRepositories())),
		JobsRunning:         overview.Msg.GetJobsRunning(),
		JobsLast7Days:       overview.Msg.GetJobsLast_7Days(),
		HasJobs:             overview.Msg.GetHasJobs(),
	}
	for _, installation := range installations.Msg.GetInstallations() {
		status.Installations = append(status.Installations, Installation{
			InstallationID:      installation.GetInstallationId(),
			AccountLogin:        installation.GetAccountLogin(),
			AccountType:         installation.GetAccountType(),
			RepositorySelection: installation.GetRepositorySelection(),
			Suspended:           installation.GetSuspended(),
		})
	}
	for _, repository := range repositories.Msg.GetRepositories() {
		status.EnabledRepositories = append(status.EnabledRepositories, repositoryFromProto(repository))
	}
	status.NextStep = nextStep(len(status.Installations), status.EnabledRepositories, status.HasJobs)
	return status, nil
}

// nextStep picks the next step from what the server reports. It never
// guesses why something is missing: no jobs only means none were seen yet.
func nextStep(installations int, repositories []Repository, hasJobs bool) NextStep {
	if installations == 0 {
		return NextStep{Code: NextStepInstallApp, Message: "The GitHub App is not installed for this tenant. Get an install link with efc_get_install_link (CLI: efc install-link) and give it to a person."}
	}
	if len(repositories) == 0 {
		return NextStep{Code: NextStepEnableRepository, Message: "No repository is enabled. List repositories with efc_list_available_repositories (CLI: efc repos available) and enable one with efc_enable_repository (CLI: efc repos enable)."}
	}
	for _, repository := range repositories {
		if repository.Status == repositoryStatusProblem || repository.Status == repositoryStatusPoolMinute {
			message := fmt.Sprintf("Repository %s has status %s.", repository.FullName, repository.Status)
			if repository.Problem != "" {
				message += " " + repository.Problem
			}
			return NextStep{Code: NextStepCheckRepository, Message: message}
		}
	}
	if !hasJobs {
		return NextStep{Code: NextStepNoJobsObservedYet, Message: noJobsObservedYetMessage}
	}
	return NextStep{Code: NextStepReady, Message: "Elastic Fruit Cloud runs jobs for this tenant."}
}

// InstallLink returns the GitHub App install link for the tenant. Only a
// person can finish the install: the link is for a person to click.
func (t *Tools) InstallLink(ctx context.Context) (InstallLink, error) {
	info, err := t.ServiceInfo(ctx)
	if err != nil {
		return InstallLink{}, err
	}
	link, err := t.client.GetInstallURL(ctx, connect.NewRequest(&cloudv1.GetInstallURLRequest{}))
	if err != nil {
		return InstallLink{}, err
	}
	return InstallLink{
		URL:              link.Msg.GetUrl(),
		ExpiresInSeconds: installLinkLifetimeSeconds,
		Instructions: []string{
			"Give this link to a person. It works for 15 minutes.",
			fmt.Sprintf("Before clicking it, the person signs in at %s with GitHub and must be a member of this tenant.", info.PublicURL),
			"The person who installs the GitHub App on GitHub must be this same person.",
			"After the install, call efc_get_status (CLI: efc status) until the installation shows up.",
		},
	}, nil
}

func quotaFromProto(quota *cloudv1.TenantQuota) Quota {
	return Quota{
		MaxRepositories:      quota.GetMaxRepositories(),
		RepositoriesEnabled:  quota.GetRepositoriesEnabled(),
		MaxConcurrentRunners: quota.GetMaxConcurrentRunners(),
		RunnersAllocated:     quota.GetRunnersAllocated(),
		PoolMinutesMonthly:   quota.GetPoolMinutesMonthly(),
		PoolMinutesUsed:      quota.GetPoolMinutesUsed(),
		MaxPoolRunners:       quota.GetMaxPoolRunners(),
		PoolRunnersAllocated: quota.GetPoolRunnersAllocated(),
		MaxAgents:            quota.GetMaxAgents(),
		AgentsUsed:           quota.GetAgentsUsed(),
	}
}
