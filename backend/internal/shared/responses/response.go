// Package responses writes the API's JSON envelopes.
package responses

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []fieldDetail `json:"details,omitempty"`
}

type fieldDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error writes the standard error envelope and stops the handler chain.
func Error(c *gin.Context, status int, code, message string, details []apperrors.FieldError) {
	payload := errorBody{Error: errorPayload{Code: code, Message: message}}
	if len(details) > 0 {
		payload.Error.Details = make([]fieldDetail, len(details))
		for i, detail := range details {
			payload.Error.Details[i] = fieldDetail{Field: detail.Field, Message: detail.Message}
		}
	}
	c.AbortWithStatusJSON(status, payload)
}

// Data writes a single-resource envelope.
func Data(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

// Health writes a status document for health checks.
func Health(c *gin.Context, status int, state string) {
	c.JSON(status, gin.H{"status": state})
}

// NoContent writes an empty success response.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
