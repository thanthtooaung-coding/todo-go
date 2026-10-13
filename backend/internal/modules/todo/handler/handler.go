// Package handler handles todo HTTP requests.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/logging"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/mapper"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/request"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/service"
	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/shared/responses"
)

// Service is the application boundary used by the todo handlers.
type Service interface {
	Create(ctx context.Context, in service.CreateInput) (models.Todo, error)
	List(ctx context.Context, in service.ListInput) (service.ListResult, error)
	Get(ctx context.Context, id uuid.UUID) (models.Todo, error)
	Update(ctx context.Context, id uuid.UUID, in service.UpdateInput) (models.Todo, error)
	SetCompletion(ctx context.Context, id uuid.UUID, completed bool) (models.Todo, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Handler serves the todo routes.
type Handler struct {
	svc Service
}

// New returns a todo HTTP handler.
func New(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Create stores a todo and returns it.
func (h *Handler) Create(c *gin.Context) {
	var req request.CreateTodo
	if !decodeJSON(c, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		h.writeError(c, err)
		return
	}
	item, err := h.svc.Create(c.Request.Context(), mapper.CreateInput(req))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.Header("Location", "/api/v1/todos/"+item.ID.String())
	responses.Data(c, http.StatusCreated, mapper.Todo(item))
}

// List returns a page of todos.
func (h *Handler) List(c *gin.Context) {
	query, err := request.ParseListTodos(c.Request.URL.Query())
	if err != nil {
		h.writeError(c, err)
		return
	}
	result, err := h.svc.List(c.Request.Context(), mapper.ListInput(query))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapper.TodoList(result))
}

// Get returns one todo.
func (h *Handler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	responses.Data(c, http.StatusOK, mapper.Todo(item))
}

// Update changes title, description, or due date.
func (h *Handler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.UpdateTodo
	if !decodeJSON(c, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		h.writeError(c, err)
		return
	}
	item, err := h.svc.Update(c.Request.Context(), id, mapper.UpdateInput(req))
	if err != nil {
		h.writeError(c, err)
		return
	}
	responses.Data(c, http.StatusOK, mapper.Todo(item))
}

// SetCompletion marks a todo complete or incomplete.
func (h *Handler) SetCompletion(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.SetCompletion
	if !decodeJSON(c, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		h.writeError(c, err)
		return
	}
	item, err := h.svc.SetCompletion(c.Request.Context(), id, *req.Completed)
	if err != nil {
		h.writeError(c, err)
		return
	}
	responses.Data(c, http.StatusOK, mapper.Todo(item))
}

// Delete removes a todo.
func (h *Handler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.writeError(c, err)
		return
	}
	responses.NoContent(c)
}

func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		responses.Error(c, http.StatusBadRequest, string(apperrors.CodeValidation), "request validation failed", []apperrors.FieldError{
			apperrors.Field("id", "id must be a UUID"),
		})
		return uuid.UUID{}, false
	}
	return id, true
}

func decodeJSON(c *gin.Context, dest any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(dest); err != nil {
		writeDecodeError(c, err)
		return false
	}
	if decoder.More() {
		responses.Error(c, http.StatusBadRequest, string(apperrors.CodeValidation), "request body must be a single JSON object", nil)
		return false
	}
	return true
}

func writeDecodeError(c *gin.Context, err error) {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		responses.Error(c, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large", nil)
		return
	}
	if errors.Is(err, io.EOF) {
		responses.Error(c, http.StatusBadRequest, string(apperrors.CodeValidation), "request body is required", nil)
		return
	}
	var decodeErr *request.DecodeError
	if errors.As(err, &decodeErr) {
		if decodeErr.Field == "" {
			responses.Error(c, http.StatusBadRequest, string(apperrors.CodeValidation), decodeErr.Message, nil)
			return
		}
		responses.Error(c, http.StatusBadRequest, string(apperrors.CodeValidation), "request validation failed", []apperrors.FieldError{
			apperrors.Field(decodeErr.Field, decodeErr.Message),
		})
		return
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) || errors.Is(err, io.ErrUnexpectedEOF) {
		responses.Error(c, http.StatusBadRequest, string(apperrors.CodeValidation), "request body must be valid JSON", nil)
		return
	}
	logging.FromContext(c.Request.Context()).Debug("rejected request body", "error", err)
	responses.Error(c, http.StatusBadRequest, string(apperrors.CodeValidation), "request body is invalid", nil)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		responses.Error(c, http.StatusGatewayTimeout, "timeout", "the request timed out", nil)
		return
	}
	if errors.Is(err, context.Canceled) {
		c.Abort()
		return
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case apperrors.CodeValidation:
			responses.Error(c, http.StatusBadRequest, string(appErr.Code), appErr.Message, appErr.Details)
		case apperrors.CodeNotFound:
			responses.Error(c, http.StatusNotFound, string(appErr.Code), appErr.Message, nil)
		case apperrors.CodeConflict:
			responses.Error(c, http.StatusConflict, string(appErr.Code), appErr.Message, nil)
		default:
			logging.FromContext(c.Request.Context()).Error("request failed", "error", err)
			responses.Error(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred", nil)
		}
		return
	}
	logging.FromContext(c.Request.Context()).Error("request failed", "error", err)
	responses.Error(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred", nil)
}
