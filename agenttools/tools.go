// Package agenttools holds the actions AI agents and the efc CLI take on
// Elastic Fruit Cloud. Every action calls the CloudService API through the
// generated Connect client, so the server checks the caller like any other
// API request. Requests leave tenant_id empty, the server uses the tenant of
// the API token. Inputs and outputs are plain structs that encode to JSON.
package agenttools

import (
	"github.com/boring-design/elastic-fruit-protocol/gen/cloud/v1/cloudv1connect"
)

// DefaultAPIURL is the Elastic Fruit Cloud API used when EFC_API_URL is not set.
const DefaultAPIURL = "https://elastic-fruit.strrl.cloud"

// RunsOnLabel is the runs-on label that sends a workflow job to Elastic Fruit Cloud.
const RunsOnLabel = "elastic-fruit"

// Tools runs the agent actions with one API client.
type Tools struct {
	client cloudv1connect.CloudServiceClient
}

// New returns the agent actions that call the API through client.
func New(client cloudv1connect.CloudServiceClient) *Tools {
	return &Tools{client: client}
}
