// Package request defines todo HTTP request DTOs and transport validation.
package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
)

// DecodeError is an invalid JSON field that is safe to show to the client.
type DecodeError struct {
	Field   string
	Message string
}

func (e *DecodeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

// CreateTodo is the body for creating a todo.
type CreateTodo struct {
	Title       string
	Description string
	DueDate     *time.Time
}

// UnmarshalJSON decodes a create body and rejects unknown or wrongly typed fields.
func (r *CreateTodo) UnmarshalJSON(data []byte) error {
	raw, err := objectFields(data)
	if err != nil {
		return err
	}
	for key := range raw {
		switch key {
		case "title", "description", "due_date":
		default:
			return &DecodeError{Field: key, Message: "is not allowed"}
		}
	}
	title, present, err := stringField(raw, "title", true)
	if err != nil {
		return err
	}
	if present {
		r.Title = title
	}
	description, _, err := stringField(raw, "description", true)
	if err != nil {
		return err
	}
	r.Description = description
	due, err := timeField(raw, "due_date", true)
	if err != nil {
		return err
	}
	r.DueDate = due
	return nil
}

// Validate normalizes the create body and reports field errors.
func (r *CreateTodo) Validate() error {
	var details []apperrors.FieldError
	title, message := models.NormalizeTitle(r.Title)
	if message != "" {
		details = append(details, apperrors.Field("title", message))
	} else {
		r.Title = title
	}
	description, message := models.NormalizeDescription(r.Description)
	if message != "" {
		details = append(details, apperrors.Field("description", message))
	} else {
		r.Description = description
	}
	if r.DueDate != nil {
		utc := r.DueDate.UTC()
		r.DueDate = &utc
	}
	if len(details) > 0 {
		return apperrors.Validation("request validation failed", details)
	}
	return nil
}

// OptionalTime records whether a nullable timestamp was present in JSON.
type OptionalTime struct {
	Set   bool
	Value *time.Time
}

// UpdateTodo is a partial update of client-editable todo fields.
type UpdateTodo struct {
	Title       *string
	Description *string
	DueDate     OptionalTime
}

// UnmarshalJSON decodes a partial update and rejects unknown fields.
func (r *UpdateTodo) UnmarshalJSON(data []byte) error {
	raw, err := objectFields(data)
	if err != nil {
		return err
	}
	for key := range raw {
		switch key {
		case "title", "description", "due_date":
		default:
			return &DecodeError{Field: key, Message: "is not allowed"}
		}
	}
	if _, ok := raw["title"]; ok {
		value, _, err := stringField(raw, "title", false)
		if err != nil {
			return err
		}
		r.Title = &value
	}
	if _, ok := raw["description"]; ok {
		value, _, err := stringField(raw, "description", false)
		if err != nil {
			return err
		}
		r.Description = &value
	}
	if _, ok := raw["due_date"]; ok {
		if isNull(raw["due_date"]) {
			r.DueDate = OptionalTime{Set: true}
			return nil
		}
		due, err := timeField(raw, "due_date", false)
		if err != nil {
			return err
		}
		r.DueDate = OptionalTime{Set: true, Value: due}
	}
	return nil
}

// Validate normalizes a partial update.
func (r *UpdateTodo) Validate() error {
	if r.Title == nil && r.Description == nil && !r.DueDate.Set {
		return apperrors.Validation("at least one of title, description, or due_date is required", nil)
	}
	var details []apperrors.FieldError
	if r.Title != nil {
		title, message := models.NormalizeTitle(*r.Title)
		if message != "" {
			details = append(details, apperrors.Field("title", message))
		} else {
			r.Title = &title
		}
	}
	if r.Description != nil {
		description, message := models.NormalizeDescription(*r.Description)
		if message != "" {
			details = append(details, apperrors.Field("description", message))
		} else {
			r.Description = &description
		}
	}
	if r.DueDate.Set && r.DueDate.Value != nil {
		utc := r.DueDate.Value.UTC()
		r.DueDate.Value = &utc
	}
	if len(details) > 0 {
		return apperrors.Validation("request validation failed", details)
	}
	return nil
}

// SetCompletion is the body for the completion endpoint.
type SetCompletion struct {
	Completed *bool
}

// UnmarshalJSON decodes the completion flag and rejects unknown fields.
func (r *SetCompletion) UnmarshalJSON(data []byte) error {
	raw, err := objectFields(data)
	if err != nil {
		return err
	}
	for key := range raw {
		if key != "completed" {
			return &DecodeError{Field: key, Message: "is not allowed"}
		}
	}
	payload, ok := raw["completed"]
	if !ok || isNull(payload) {
		return &DecodeError{Field: "completed", Message: "is required"}
	}
	var value bool
	if err := json.Unmarshal(payload, &value); err != nil {
		return &DecodeError{Field: "completed", Message: "must be a boolean"}
	}
	r.Completed = &value
	return nil
}

// Validate reports a missing completion flag.
func (r SetCompletion) Validate() error {
	if r.Completed == nil {
		return apperrors.Validation("request validation failed", []apperrors.FieldError{
			apperrors.Field("completed", "completed is required"),
		})
	}
	return nil
}

// ListTodos is a validated list query.
type ListTodos struct {
	Page      int
	PageSize  int
	Completed *bool
	Query     string
	Sort      string
	Order     string
}

// ParseListTodos validates list query parameters and applies defaults.
func ParseListTodos(values url.Values) (ListTodos, error) {
	var details []apperrors.FieldError
	query := ListTodos{
		Page:     models.DefaultPage,
		PageSize: models.DefaultPageSize,
		Sort:     models.DefaultSort,
		Order:    models.DefaultOrder,
	}

	if raw := values.Get("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 || page > models.MaxPage {
			details = append(details, apperrors.Field("page", "page must be an integer from 1 to 10000"))
		} else {
			query.Page = page
		}
	}
	if raw := values.Get("page_size"); raw != "" {
		pageSize, err := strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > models.MaxPageSize {
			details = append(details, apperrors.Field("page_size", "page_size must be an integer from 1 to 100"))
		} else {
			query.PageSize = pageSize
		}
	}
	if raw := values.Get("completed"); raw != "" {
		switch raw {
		case "true":
			value := true
			query.Completed = &value
		case "false":
			value := false
			query.Completed = &value
		default:
			details = append(details, apperrors.Field("completed", "completed must be true or false"))
		}
	}
	if raw := values.Get("q"); raw != "" {
		normalized, message := models.NormalizeQuery(raw)
		if message != "" {
			details = append(details, apperrors.Field("q", message))
		} else {
			query.Query = normalized
		}
	}
	if raw := values.Get("sort"); raw != "" {
		switch raw {
		case models.SortCreatedAt, models.SortDueDate, models.SortTitle:
			query.Sort = raw
		default:
			details = append(details, apperrors.Field("sort", "sort must be one of created_at, due_date, or title"))
		}
	}
	if raw := values.Get("order"); raw != "" {
		switch raw {
		case models.OrderAsc, models.OrderDesc:
			query.Order = raw
		default:
			details = append(details, apperrors.Field("order", "order must be asc or desc"))
		}
	}
	if len(details) > 0 {
		return ListTodos{}, apperrors.Validation("request validation failed", details)
	}
	return query, nil
}

func objectFields(data []byte) (map[string]json.RawMessage, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid json")
	}
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&raw); err != nil {
		return nil, &DecodeError{Message: "request body must be a JSON object"}
	}
	return raw, nil
}

func stringField(raw map[string]json.RawMessage, key string, optional bool) (string, bool, error) {
	payload, ok := raw[key]
	if !ok {
		if optional {
			return "", false, nil
		}
		return "", false, nil
	}
	if isNull(payload) {
		return "", false, &DecodeError{Field: key, Message: "must be a string"}
	}
	var value string
	if err := json.Unmarshal(payload, &value); err != nil {
		return "", false, &DecodeError{Field: key, Message: "must be a string"}
	}
	return value, true, nil
}

func timeField(raw map[string]json.RawMessage, key string, nullAllowed bool) (*time.Time, error) {
	payload, ok := raw[key]
	if !ok || (nullAllowed && isNull(payload)) {
		return nil, nil
	}
	if isNull(payload) {
		return nil, &DecodeError{Field: key, Message: "must be an RFC3339 timestamp or null"}
	}
	var value string
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, &DecodeError{Field: key, Message: "must be an RFC3339 timestamp or null"}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, &DecodeError{Field: key, Message: "must be an RFC3339 timestamp or null"}
	}
	utc := parsed.UTC()
	return &utc, nil
}

func isNull(data []byte) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}
