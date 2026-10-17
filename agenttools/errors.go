package agenttools

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
)

// ErrorText describes an error for an agent or a user. An API error shows
// only its code and the message of the server, the cause is not guessed:
// permission_denied can come from the token scope or from the tenant.
func ErrorText(err error) string {
	var apiErr *connect.Error
	if !errors.As(err, &apiErr) {
		return err.Error()
	}
	text := fmt.Sprintf("EFC API error %s: %s", apiErr.Code(), apiErr.Message())
	if apiErr.Code() == connect.CodeUnauthenticated {
		text += " (check EFC_TOKEN)"
	}
	return text
}
