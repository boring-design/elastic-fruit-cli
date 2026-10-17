package agenttools

import (
	"context"

	"connectrpc.com/connect"

	cloudv1 "github.com/boring-design/elastic-fruit-protocol/gen/cloud/v1"
)

// defaultUsageDays is the report length when none is given.
const defaultUsageDays = 30

// Usage is the daily usage of the tenant per repository, with totals.
type Usage struct {
	// From and To are the first and last UTC day of the report as 2026-01-31, both included.
	From   string      `json:"from"`
	To     string      `json:"to"`
	Totals UsageTotals `json:"totals"`
	Rows   []UsageRow  `json:"rows"`
}

// UsageTotals sums up the rows of a usage report.
type UsageTotals struct {
	JobCount          int32 `json:"job_count"`
	SucceededCount    int32 `json:"succeeded_count"`
	FailedCount       int32 `json:"failed_count"`
	OtherCount        int32 `json:"other_count"`
	JobSeconds        int64 `json:"job_seconds"`
	RunnerSeconds     int64 `json:"runner_seconds"`
	PoolRunnerSeconds int64 `json:"pool_runner_seconds"`
}

// UsageRow is the usage of one repository on one UTC day.
type UsageRow struct {
	Day               string `json:"day"`
	Repository        string `json:"repository"`
	JobCount          int32  `json:"job_count"`
	SucceededCount    int32  `json:"succeeded_count"`
	FailedCount       int32  `json:"failed_count"`
	OtherCount        int32  `json:"other_count"`
	JobSeconds        int64  `json:"job_seconds"`
	RunnerSeconds     int64  `json:"runner_seconds"`
	PoolRunnerSeconds int64  `json:"pool_runner_seconds"`
}

// Usage reads the usage of the last days, today included. days 0 means 30,
// the server allows 1 to 366.
func (t *Tools) Usage(ctx context.Context, days int32) (Usage, error) {
	if days == 0 {
		days = defaultUsageDays
	}
	response, err := t.client.GetUsageReport(ctx, connect.NewRequest(&cloudv1.GetUsageReportRequest{Days: days}))
	if err != nil {
		return Usage{}, err
	}
	totals := response.Msg.GetTotals()
	report := Usage{
		From: response.Msg.GetFrom(),
		To:   response.Msg.GetTo(),
		Totals: UsageTotals{
			JobCount:          totals.GetJobCount(),
			SucceededCount:    totals.GetSucceededCount(),
			FailedCount:       totals.GetFailedCount(),
			OtherCount:        totals.GetOtherCount(),
			JobSeconds:        totals.GetJobSeconds(),
			RunnerSeconds:     totals.GetRunnerSeconds(),
			PoolRunnerSeconds: totals.GetPoolRunnerSeconds(),
		},
		Rows: make([]UsageRow, 0, len(response.Msg.GetRows())),
	}
	for _, row := range response.Msg.GetRows() {
		report.Rows = append(report.Rows, UsageRow{
			Day:               row.GetDay(),
			Repository:        row.GetRepositoryName(),
			JobCount:          row.GetJobCount(),
			SucceededCount:    row.GetSucceededCount(),
			FailedCount:       row.GetFailedCount(),
			OtherCount:        row.GetOtherCount(),
			JobSeconds:        row.GetJobSeconds(),
			RunnerSeconds:     row.GetRunnerSeconds(),
			PoolRunnerSeconds: row.GetPoolRunnerSeconds(),
		})
	}
	return report, nil
}
