// Package repository stores todos in PostgreSQL through GORM.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
)

// ListParams is a validated list query. Sort and Order must already be allowlisted.
type ListParams struct {
	Page      int
	PageSize  int
	Completed *bool
	Query     string
	Sort      string
	Order     string
}

// ListResult is one page of todos and the unpaged total.
type ListResult struct {
	Items []models.Todo
	Total int64
}

// Repository performs todo persistence operations.
type Repository struct {
	db *gorm.DB
}

// New returns a repository that uses the given database handle.
func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a todo and fills its ID and timestamps when they are empty.
func (r *Repository) Create(ctx context.Context, todo *models.Todo) error {
	if todo == nil {
		return fmt.Errorf("create todo: todo is nil")
	}
	if todo.ID == uuid.Nil {
		todo.ID = uuid.New()
	}
	now := time.Now().UTC()
	if todo.CreatedAt.IsZero() {
		todo.CreatedAt = now
	}
	if todo.UpdatedAt.IsZero() {
		todo.UpdatedAt = now
	}
	err := r.db.WithContext(ctx).
		Select("id", "title", "description", "completed", "due_date", "created_at", "updated_at").
		Create(todo).Error
	if err != nil {
		return mapDBError(err, "create todo")
	}
	return nil
}

// GetByID returns one todo by primary key.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (models.Todo, error) {
	var todo models.Todo
	err := r.db.WithContext(ctx).First(&todo, "id = ?", id).Error
	if err != nil {
		return models.Todo{}, mapDBError(err, "get todo")
	}
	return todo, nil
}

// List returns a filtered, sorted page of todos.
func (r *Repository) List(ctx context.Context, params ListParams) (ListResult, error) {
	orderBy, err := orderClause(params.Sort, params.Order)
	if err != nil {
		return ListResult{}, err
	}
	if params.Page < 1 || params.Page > models.MaxPage || params.PageSize < 1 || params.PageSize > models.MaxPageSize {
		return ListResult{}, apperrors.Validation("request validation failed", []apperrors.FieldError{
			apperrors.Field("page", "page or page_size is invalid"),
		})
	}

	filtered := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.Todo{})
		if params.Completed != nil {
			query = query.Where("completed = ?", *params.Completed)
		}
		if params.Query != "" {
			pattern := "%" + escapeLike(params.Query) + "%"
			query = query.Where("title ILIKE ? ESCAPE '\\'", pattern)
		}
		return query
	}

	var total int64
	if err := filtered(r.db.WithContext(ctx)).Count(&total).Error; err != nil {
		return ListResult{}, mapDBError(err, "count todos")
	}

	var items []models.Todo
	err = filtered(r.db.WithContext(ctx)).
		Order(orderBy).
		Limit(params.PageSize).
		Offset((params.Page - 1) * params.PageSize).
		Find(&items).Error
	if err != nil {
		return ListResult{}, mapDBError(err, "list todos")
	}
	if items == nil {
		items = []models.Todo{}
	}
	return ListResult{Items: items, Total: total}, nil
}

// Update writes the editable columns, including empty and false values.
func (r *Repository) Update(ctx context.Context, todo *models.Todo) error {
	if todo == nil {
		return fmt.Errorf("update todo: todo is nil")
	}
	todo.UpdatedAt = time.Now().UTC()
	tx := r.db.WithContext(ctx).Model(&models.Todo{}).Where("id = ?", todo.ID).Updates(map[string]any{
		"title":       todo.Title,
		"description": todo.Description,
		"completed":   todo.Completed,
		"due_date":    todo.DueDate,
		"updated_at":  todo.UpdatedAt,
	})
	if tx.Error != nil {
		return mapDBError(tx.Error, "update todo")
	}
	if tx.RowsAffected == 0 {
		return apperrors.NotFound("todo not found")
	}
	return nil
}

// Delete removes a todo by primary key.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tx := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Todo{})
	if tx.Error != nil {
		return mapDBError(tx.Error, "delete todo")
	}
	if tx.RowsAffected == 0 {
		return apperrors.NotFound("todo not found")
	}
	return nil
}

// orderClause maps allowlisted sort options to a fixed SQL fragment.
// Callers must not pass untrusted column names through this function.
func orderClause(sort, order string) (string, error) {
	columns := map[string]string{
		models.SortCreatedAt: "created_at",
		models.SortDueDate:   "due_date",
		models.SortTitle:     "title",
	}
	column, ok := columns[sort]
	if !ok {
		return "", apperrors.Validation("request validation failed", []apperrors.FieldError{
			apperrors.Field("sort", "sort must be one of created_at, due_date, or title"),
		})
	}
	var direction string
	switch order {
	case models.OrderAsc:
		direction = "ASC"
	case models.OrderDesc:
		direction = "DESC"
	default:
		return "", apperrors.Validation("request validation failed", []apperrors.FieldError{
			apperrors.Field("order", "order must be asc or desc"),
		})
	}
	return column + " " + direction + " NULLS LAST", nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func mapDBError(err error, op string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperrors.NotFound("todo not found")
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return apperrors.Conflict("todo already exists")
		case "23514":
			return checkConstraintError(pgErr.ConstraintName)
		case "22001":
			return apperrors.Validation("request validation failed", []apperrors.FieldError{
				apperrors.Field("request", "a field exceeds the allowed length"),
			})
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func checkConstraintError(name string) error {
	switch name {
	case "todos_title_not_blank":
		return apperrors.Validation("request validation failed", []apperrors.FieldError{
			apperrors.Field("title", "title is required"),
		})
	case "todos_description_length":
		return apperrors.Validation("request validation failed", []apperrors.FieldError{
			apperrors.Field("description", "description must be at most 2000 characters"),
		})
	default:
		return apperrors.Validation("request validation failed", nil)
	}
}
