package agenttools

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	cloudv1 "github.com/boring-design/elastic-fruit-protocol/gen/cloud/v1"
)

// Repository is one enabled repository of the tenant.
type Repository struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	Archived bool   `json:"archived"`
	// Status is one of paused, problem, setting_up, pool_minutes_used_up or ready.
	Status        string         `json:"status"`
	Problem       string         `json:"problem,omitempty"`
	LastJob       *RepositoryJob `json:"last_job,omitempty"`
	JobsLast7Days int32          `json:"jobs_last_7_days"`
}

// RepositoryJob is the newest job of a repository.
type RepositoryJob struct {
	ID         string `json:"id"`
	Result     string `json:"result"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// RepositoryList is the enabled repositories of the tenant.
type RepositoryList struct {
	Repositories []Repository `json:"repositories"`
}

// AvailableRepository is one repository a GitHub App installation of the tenant can see.
type AvailableRepository struct {
	ID             int64  `json:"id"`
	FullName       string `json:"full_name"`
	InstallationID int64  `json:"installation_id"`
	AccountLogin   string `json:"account_login"`
	Private        bool   `json:"private"`
	Archived       bool   `json:"archived"`
	Enabled        bool   `json:"enabled"`
}

// AvailableRepositoryList is the repositories the installations of the tenant can see.
type AvailableRepositoryList struct {
	Repositories []AvailableRepository `json:"repositories"`
}

// EnabledRepository is the answer to enabling a repository.
type EnabledRepository struct {
	Repository Repository `json:"repository"`
	RunsOn     string     `json:"runs_on"`
	Message    string     `json:"message"`
}

// DisabledRepository is the answer to disabling a repository.
type DisabledRepository struct {
	FullName string `json:"full_name"`
	Disabled bool   `json:"disabled"`
}

// Repositories lists the enabled repositories of the tenant.
func (t *Tools) Repositories(ctx context.Context) (RepositoryList, error) {
	response, err := t.client.ListRepositories(ctx, connect.NewRequest(&cloudv1.ListRepositoriesRequest{}))
	if err != nil {
		return RepositoryList{}, err
	}
	list := RepositoryList{Repositories: make([]Repository, 0, len(response.Msg.GetRepositories()))}
	for _, repository := range response.Msg.GetRepositories() {
		list.Repositories = append(list.Repositories, repositoryFromProto(repository))
	}
	return list, nil
}

// AvailableRepositories lists the repositories the GitHub App installations
// of the tenant can see. A non empty account keeps only that GitHub account.
func (t *Tools) AvailableRepositories(ctx context.Context, account string) (AvailableRepositoryList, error) {
	response, err := t.client.ListInstallationRepositories(ctx, connect.NewRequest(&cloudv1.ListInstallationRepositoriesRequest{}))
	if err != nil {
		return AvailableRepositoryList{}, err
	}
	list := AvailableRepositoryList{Repositories: []AvailableRepository{}}
	for _, repository := range response.Msg.GetRepositories() {
		if account != "" && !strings.EqualFold(repository.GetAccountLogin(), account) {
			continue
		}
		list.Repositories = append(list.Repositories, AvailableRepository{
			ID:             repository.GetId(),
			FullName:       repository.GetFullName(),
			InstallationID: repository.GetInstallationId(),
			AccountLogin:   repository.GetAccountLogin(),
			Private:        repository.GetPrivate(),
			Archived:       repository.GetArchived(),
			Enabled:        repository.GetEnabled(),
		})
	}
	return list, nil
}

// EnableRepository enables the repository named owner/name. It needs an
// admin token. The repository must be visible to an installation of the tenant.
func (t *Tools) EnableRepository(ctx context.Context, fullName string) (EnabledRepository, error) {
	available, err := t.client.ListInstallationRepositories(ctx, connect.NewRequest(&cloudv1.ListInstallationRepositoriesRequest{}))
	if err != nil {
		return EnabledRepository{}, err
	}
	found, err := findAvailableRepository(available.Msg.GetRepositories(), fullName)
	if err != nil {
		return EnabledRepository{}, err
	}
	response, err := t.client.EnableRepository(ctx, connect.NewRequest(&cloudv1.EnableRepositoryRequest{
		RepositoryId:   found.GetId(),
		InstallationId: found.GetInstallationId(),
	}))
	if err != nil {
		return EnabledRepository{}, err
	}
	repository := repositoryFromProto(response.Msg.GetRepository())
	return EnabledRepository{
		Repository: repository,
		RunsOn:     RunsOnLabel,
		Message:    fmt.Sprintf("Workflow jobs of %s with `runs-on: %s` run on Elastic Fruit Cloud.", repository.FullName, RunsOnLabel),
	}, nil
}

// DisableRepository disables the enabled repository named owner/name. It
// needs an admin token. Job history stays.
func (t *Tools) DisableRepository(ctx context.Context, fullName string) (DisabledRepository, error) {
	enabled, err := t.client.ListRepositories(ctx, connect.NewRequest(&cloudv1.ListRepositoriesRequest{}))
	if err != nil {
		return DisabledRepository{}, err
	}
	found, err := findEnabledRepository(enabled.Msg.GetRepositories(), fullName)
	if err != nil {
		return DisabledRepository{}, err
	}
	if _, err := t.client.DisableRepository(ctx, connect.NewRequest(&cloudv1.DisableRepositoryRequest{RepositoryId: found.GetId()})); err != nil {
		return DisabledRepository{}, err
	}
	return DisabledRepository{FullName: found.GetFullName(), Disabled: true}, nil
}

// findAvailableRepository finds owner/name among the repositories the
// installations can see. GitHub names are not case sensitive.
func findAvailableRepository(repositories []*cloudv1.InstallationRepository, fullName string) (*cloudv1.InstallationRepository, error) {
	var matches []*cloudv1.InstallationRepository
	for _, repository := range repositories {
		if strings.EqualFold(repository.GetFullName(), fullName) {
			matches = append(matches, repository)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("repository %q is not visible to any GitHub App installation of this tenant, check the name as owner/name or give the App access to it on GitHub", fullName)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("repository %q is visible through %d GitHub App installations of this tenant, enable it in the dashboard", fullName, len(matches))
	}
}

// findEnabledRepository finds owner/name among the enabled repositories.
func findEnabledRepository(repositories []*cloudv1.Repository, fullName string) (*cloudv1.Repository, error) {
	for _, repository := range repositories {
		if strings.EqualFold(repository.GetFullName(), fullName) {
			return repository, nil
		}
	}
	return nil, fmt.Errorf("repository %q is not enabled in this tenant", fullName)
}

func repositoryFromProto(repository *cloudv1.Repository) Repository {
	result := Repository{
		ID:            repository.GetId(),
		FullName:      repository.GetFullName(),
		Private:       repository.GetPrivate(),
		Archived:      repository.GetArchived(),
		Status:        repository.GetStatus(),
		Problem:       repository.GetProblem(),
		JobsLast7Days: repository.GetJobsLast_7Days(),
	}
	if job := repository.GetLastJob(); job != nil {
		result.LastJob = &RepositoryJob{
			ID:         job.GetId(),
			Result:     resultOrRunning(job.GetResult()),
			StartedAt:  job.GetStartedAt(),
			FinishedAt: job.GetFinishedAt(),
		}
	}
	return result
}
