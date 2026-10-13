// Package service implements todo application operations.
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/repository"
	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
)

// Store is the persistence boundary used by the todo service.
type Store interface {
	Create(ctx context.Context, todo *models.Todo) error
	GetByID(ctx context.Context, id uuid.UUID) (models.Todo, error)
	List(ctx context.Context, params repository.ListParams) (repository.ListResult, error)
	Update(ctx context.Context, todo *models.Todo) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// CreateInput is the data required to create a todo.
type CreateInput struct {
	Title       string
	Description string
	DueDate     *time.Time
}

// OptionalTime records a nullable due date change.
type OptionalTime struct {
	Set   bool
	Value *time.Time
}

// UpdateInput is a partial change to client-editable fields.
type UpdateInput struct {
	Title       *string
	Description *string
	DueDate     OptionalTime
}

// ListInput is a validated todo list query.
type ListInput struct {
	Page      int
	PageSize  int
	Completed *bool
	Query     string
	Sort      string
	Order     string
}

// ListResult is one page of todos.
type ListResult struct {
	Items    []models.Todo
	Total    int64
	Page     int
	PageSize int
}

// Service coordinates todo operations.
type Service struct {
	store Store
}

// New returns a todo service.
func New(store Store) *Service {
	return &Service{store: store}
}

// Create stores a new incomplete todo.
func (s *Service) Create(ctx context.Context, in CreateInput) (models.Todo, error) {
	title, description, details := normalizeRequiredText(in.Title, in.Description)
	if len(details) > 0 {
		return models.Todo{}, apperrors.Validation("request validation failed", details)
	}
	due := normalizeDueDate(in.DueDate)
	todo := &models.Todo{
		Title:       title,
		Description: description,
		Completed:   false,
		DueDate:     due,
	}
	if err := s.store.Create(ctx, todo); err != nil {
		return models.Todo{}, err
	}
	return *todo, nil
}

// Get returns one todo.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (models.Todo, error) {
	return s.store.GetByID(ctx, id)
}

// List returns a page of todos.
func (s *Service) List(ctx context.Context, in ListInput) (ListResult, error) {
	normalized, err := normalizeList(in)
	if err != nil {
		return ListResult{}, err
	}
	page, err := s.store.List(ctx, repository.ListParams{
		Page:      normalized.Page,
		PageSize:  normalized.PageSize,
		Completed: normalized.Completed,
		Query:     normalized.Query,
		Sort:      normalized.Sort,
		Order:     normalized.Order,
	})
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{
		Items:    page.Items,
		Total:    page.Total,
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
	}, nil
}

// Update changes title, description, or due date without changing completion.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (models.Todo, error) {
	if in.Title == nil && in.Description == nil && !in.DueDate.Set {
		return models.Todo{}, apperrors.Validation("at least one of title, description, or due_date is required", nil)
	}
	var details []apperrors.FieldError
	if in.Title != nil {
		title, message := models.NormalizeTitle(*in.Title)
		if message != "" {
			details = append(details, apperrors.Field("title", message))
		} else {
			in.Title = &title
		}
	}
	if in.Description != nil {
		description, message := models.NormalizeDescription(*in.Description)
		if message != "" {
			details = append(details, apperrors.Field("description", message))
		} else {
			in.Description = &description
		}
	}
	if len(details) > 0 {
		return models.Todo{}, apperrors.Validation("request validation failed", details)
	}
	if in.DueDate.Set {
		in.DueDate.Value = normalizeDueDate(in.DueDate.Value)
	}

	todo, err := s.store.GetByID(ctx, id)
	if err != nil {
		return models.Todo{}, err
	}
	if in.Title != nil {
		todo.Title = *in.Title
	}
	if in.Description != nil {
		todo.Description = *in.Description
	}
	if in.DueDate.Set {
		todo.DueDate = in.DueDate.Value
	}
	if err := s.store.Update(ctx, &todo); err != nil {
		return models.Todo{}, err
	}
	return todo, nil
}

// SetCompletion marks a todo complete or incomplete.
func (s *Service) SetCompletion(ctx context.Context, id uuid.UUID, completed bool) (models.Todo, error) {
	todo, err := s.store.GetByID(ctx, id)
	if err != nil {
		return models.Todo{}, err
	}
	todo.Completed = completed
	if err := s.store.Update(ctx, &todo); err != nil {
		return models.Todo{}, err
	}
	return todo, nil
}

// Delete removes a todo.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Delete(ctx, id)
}

func normalizeRequiredText(title, description string) (string, string, []apperrors.FieldError) {
	var details []apperrors.FieldError
	normalizedTitle, titleMessage := models.NormalizeTitle(title)
	if titleMessage != "" {
		details = append(details, apperrors.Field("title", titleMessage))
	}
	normalizedDescription, descriptionMessage := models.NormalizeDescription(description)
	if descriptionMessage != "" {
		details = append(details, apperrors.Field("description", descriptionMessage))
	}
	return normalizedTitle, normalizedDescription, details
}

func normalizeDueDate(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func normalizeList(in ListInput) (ListInput, error) {
	var details []apperrors.FieldError
	if in.Page == 0 {
		in.Page = models.DefaultPage
	}
	if in.Page < 1 || in.Page > models.MaxPage {
		details = append(details, apperrors.Field("page", "page must be an integer from 1 to 10000"))
	}
	if in.PageSize == 0 {
		in.PageSize = models.DefaultPageSize
	}
	if in.PageSize < 1 || in.PageSize > models.MaxPageSize {
		details = append(details, apperrors.Field("page_size", "page_size must be an integer from 1 to 100"))
	}
	query, message := models.NormalizeQuery(in.Query)
	if message != "" {
		details = append(details, apperrors.Field("q", message))
	} else {
		in.Query = query
	}
	if in.Sort == "" {
		in.Sort = models.DefaultSort
	}
	switch in.Sort {
	case models.SortCreatedAt, models.SortDueDate, models.SortTitle:
	default:
		details = append(details, apperrors.Field("sort", "sort must be one of created_at, due_date, or title"))
	}
	if in.Order == "" {
		in.Order = models.DefaultOrder
	}
	switch in.Order {
	case models.OrderAsc, models.OrderDesc:
	default:
		details = append(details, apperrors.Field("order", "order must be asc or desc"))
	}
	if len(details) > 0 {
		return ListInput{}, apperrors.Validation("request validation failed", details)
	}
	return in, nil
}
