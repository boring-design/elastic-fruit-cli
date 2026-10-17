package agenttools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	cloudv1 "github.com/boring-design/elastic-fruit-protocol/gen/cloud/v1"
)

// maxScannedJobPages bounds how many pages of 50 jobs a filtered job list
// reads, the server filters only by repository.
const maxScannedJobPages = 5

// resultRunning stands for a job without a result yet.
const resultRunning = "running"

// jobResults are the results a job list can be filtered by.
var jobResults = []string{"succeeded", "failed", "canceled", "lost", resultRunning}

// Job is one job of the tenant.
type Job struct {
	ID            string `json:"id"`
	Repository    string `json:"repository"`
	DisplayName   string `json:"display_name"`
	WorkflowRunID int64  `json:"workflow_run_id"`
	WorkflowRef   string `json:"workflow_ref"`
	EventName     string `json:"event_name"`
	// Result is succeeded, failed, canceled, lost or running.
	Result       string `json:"result"`
	QueuedAt     string `json:"queued_at,omitempty"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at,omitempty"`
	RunnerName   string `json:"runner_name"`
	AgentName    string `json:"agent_name"`
	GitHubRunURL string `json:"github_run_url,omitempty"`
}

// JobFilter picks the jobs of a job list. Zero values do not filter.
type JobFilter struct {
	// Repository is owner/name.
	Repository    string
	WorkflowRunID int64
	Result        string
	// Page starts at 1 and is only used without WorkflowRunID and Result.
	Page int
}

// JobList is one page of jobs, newest first.
type JobList struct {
	Jobs    []Job  `json:"jobs"`
	HasMore bool   `json:"has_more"`
	Message string `json:"message,omitempty"`
}

// JobDetail is one job with a summary of its resource samples.
type JobDetail struct {
	Job             Job            `json:"job"`
	Labels          []string       `json:"labels"`
	RunnerState     string         `json:"runner_state,omitempty"`
	RunnerEndReason string         `json:"runner_end_reason,omitempty"`
	Samples         SamplesSummary `json:"samples_summary"`
	LogsHint        LogsHint       `json:"logs_hint"`
}

// SamplesSummary sums up the resource samples of a job.
type SamplesSummary struct {
	Count              int     `json:"count"`
	MaxCPUPercent      float64 `json:"max_cpu_percent"`
	MaxMemoryUsedBytes int64   `json:"max_memory_used_bytes"`
	MaxDiskUsedBytes   int64   `json:"max_disk_used_bytes"`
	FirstRecordedAt    string  `json:"first_recorded_at,omitempty"`
	LastRecordedAt     string  `json:"last_recorded_at,omitempty"`
}

// LogsHint says where the log of a job is.
type LogsHint struct {
	Text         string `json:"text"`
	GitHubRunURL string `json:"github_run_url,omitempty"`
	Command      string `json:"command,omitempty"`
}

// JobLogs is the end of the GitHub log of one job, or why it cannot be read.
type JobLogs struct {
	// Status is ready, log_not_ready, permission_not_approved,
	// repository_not_accessible or job_not_located.
	Status              string `json:"status"`
	Message             string `json:"message,omitempty"`
	Log                 string `json:"log,omitempty"`
	Truncated           bool   `json:"truncated"`
	LogSizeBytes        int64  `json:"log_size_bytes"`
	GitHubWorkflowJobID int64  `json:"github_workflow_job_id,omitempty"`
	GitHubRunURL        string `json:"github_run_url,omitempty"`
	GitHubJobURL        string `json:"github_job_url,omitempty"`
	LogCommand          string `json:"log_command,omitempty"`
}

// Jobs lists jobs of the tenant, newest first. Without run or result filter
// it returns one page from the server. With one, it reads up to
// maxScannedJobPages pages and returns the jobs that match.
func (t *Tools) Jobs(ctx context.Context, filter JobFilter) (JobList, error) {
	if filter.Result != "" && !validResult(filter.Result) {
		return JobList{}, fmt.Errorf("result %q is not one of %s", filter.Result, strings.Join(jobResults, ", "))
	}
	var repositoryID int64
	if filter.Repository != "" {
		enabled, err := t.client.ListRepositories(ctx, connect.NewRequest(&cloudv1.ListRepositoriesRequest{}))
		if err != nil {
			return JobList{}, err
		}
		found, err := findEnabledRepository(enabled.Msg.GetRepositories(), filter.Repository)
		if err != nil {
			return JobList{}, err
		}
		repositoryID = found.GetId()
	}
	if filter.WorkflowRunID == 0 && filter.Result == "" {
		page := max(filter.Page, 1)
		response, err := t.client.ListJobs(ctx, connect.NewRequest(&cloudv1.ListJobsRequest{RepositoryId: repositoryID, Page: int32(page)}))
		if err != nil {
			return JobList{}, err
		}
		list := JobList{Jobs: jobsFromProto(response.Msg.GetJobs()), HasMore: response.Msg.GetHasMore()}
		if len(list.Jobs) == 0 && page == 1 {
			list.Message = noJobsObservedYetMessage
		}
		return list, nil
	}

	list := JobList{Jobs: []Job{}}
	for page := 1; page <= maxScannedJobPages; page++ {
		response, err := t.client.ListJobs(ctx, connect.NewRequest(&cloudv1.ListJobsRequest{RepositoryId: repositoryID, Page: int32(page)}))
		if err != nil {
			return JobList{}, err
		}
		list.Jobs = append(list.Jobs, filterJobs(jobsFromProto(response.Msg.GetJobs()), filter)...)
		if !response.Msg.GetHasMore() {
			return list, nil
		}
		list.HasMore = page == maxScannedJobPages
	}
	list.Message = fmt.Sprintf("Only the newest %d jobs were searched.", maxScannedJobPages*50)
	return list, nil
}

// Job reads one job with its resource samples and where its log is.
func (t *Tools) Job(ctx context.Context, jobID string) (JobDetail, error) {
	response, err := t.client.GetJob(ctx, connect.NewRequest(&cloudv1.GetJobRequest{JobId: jobID}))
	if err != nil {
		return JobDetail{}, err
	}
	job := jobFromProto(response.Msg.GetJob())
	return JobDetail{
		Job:             job,
		Labels:          response.Msg.GetJob().GetLabels(),
		RunnerState:     response.Msg.GetJob().GetRunnerState(),
		RunnerEndReason: response.Msg.GetJob().GetRunnerEndReason(),
		Samples:         summarizeSamples(response.Msg.GetSamples()),
		LogsHint:        logsHint(job),
	}, nil
}

// JobLogs reads the end of the GitHub log of one job. maxBytes 0 lets the
// server pick its default of 32768 bytes, it allows at most 1048576.
func (t *Tools) JobLogs(ctx context.Context, jobID string, maxBytes int32) (JobLogs, error) {
	response, err := t.client.GetJobLogs(ctx, connect.NewRequest(&cloudv1.GetJobLogsRequest{JobId: jobID, MaxBytes: maxBytes}))
	if err != nil {
		return JobLogs{}, err
	}
	return JobLogs{
		Status:              response.Msg.GetStatus(),
		Message:             response.Msg.GetMessage(),
		Log:                 response.Msg.GetLog(),
		Truncated:           response.Msg.GetTruncated(),
		LogSizeBytes:        response.Msg.GetLogSizeBytes(),
		GitHubWorkflowJobID: response.Msg.GetGithubWorkflowJobId(),
		GitHubRunURL:        response.Msg.GetGithubRunUrl(),
		GitHubJobURL:        response.Msg.GetGithubJobUrl(),
		LogCommand:          response.Msg.GetLogCommand(),
	}, nil
}

func validResult(result string) bool {
	for _, known := range jobResults {
		if result == known {
			return true
		}
	}
	return false
}

// filterJobs keeps the jobs of the run and with the result of the filter.
func filterJobs(jobs []Job, filter JobFilter) []Job {
	kept := []Job{}
	for _, job := range jobs {
		if filter.WorkflowRunID != 0 && job.WorkflowRunID != filter.WorkflowRunID {
			continue
		}
		if filter.Result != "" && job.Result != filter.Result {
			continue
		}
		kept = append(kept, job)
	}
	return kept
}

// logsHint points to the log of the job on GitHub.
func logsHint(job Job) LogsHint {
	hint := LogsHint{Text: "Logs are on GitHub. Read the end of the log with efc_get_job_logs (CLI: efc jobs logs), or with the GitHub CLI.", GitHubRunURL: job.GitHubRunURL}
	if job.GitHubRunURL != "" {
		hint.Command = "gh run view " + strconv.FormatInt(job.WorkflowRunID, 10) + " --repo " + job.Repository + " --log"
	}
	return hint
}

func summarizeSamples(samples []*cloudv1.JobResourceSample) SamplesSummary {
	summary := SamplesSummary{Count: len(samples)}
	for _, sample := range samples {
		summary.MaxCPUPercent = max(summary.MaxCPUPercent, sample.GetCpuPercent())
		summary.MaxMemoryUsedBytes = max(summary.MaxMemoryUsedBytes, sample.GetMemoryUsedBytes())
		summary.MaxDiskUsedBytes = max(summary.MaxDiskUsedBytes, sample.GetDiskUsedBytes())
	}
	if len(samples) > 0 {
		summary.FirstRecordedAt = samples[0].GetRecordedAt()
		summary.LastRecordedAt = samples[len(samples)-1].GetRecordedAt()
	}
	return summary
}

func jobsFromProto(jobs []*cloudv1.Job) []Job {
	result := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, jobFromProto(job))
	}
	return result
}

func jobFromProto(job *cloudv1.Job) Job {
	result := Job{
		ID:            job.GetId(),
		Repository:    job.GetRepositoryName(),
		DisplayName:   job.GetDisplayName(),
		WorkflowRunID: job.GetWorkflowRunId(),
		WorkflowRef:   job.GetWorkflowRef(),
		EventName:     job.GetEventName(),
		Result:        resultOrRunning(job.GetResult()),
		QueuedAt:      job.GetQueuedAt(),
		StartedAt:     job.GetStartedAt(),
		FinishedAt:    job.GetFinishedAt(),
		RunnerName:    job.GetRunnerName(),
		AgentName:     job.GetAgentName(),
	}
	if result.Repository != "" && result.WorkflowRunID != 0 {
		result.GitHubRunURL = "https://github.com/" + result.Repository + "/actions/runs/" + strconv.FormatInt(result.WorkflowRunID, 10)
	}
	return result
}

func resultOrRunning(result string) string {
	if result == "" {
		return resultRunning
	}
	return result
}
