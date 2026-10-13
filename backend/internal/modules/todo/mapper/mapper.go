// Package mapper converts todo DTOs, service inputs, and persistence models.
package mapper

import (
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/request"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/response"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/service"
)

// CreateInput maps a create request to service input.
func CreateInput(req request.CreateTodo) service.CreateInput {
	return service.CreateInput{
		Title:       req.Title,
		Description: req.Description,
		DueDate:     req.DueDate,
	}
}

// UpdateInput maps a partial update request to service input.
func UpdateInput(req request.UpdateTodo) service.UpdateInput {
	return service.UpdateInput{
		Title:       req.Title,
		Description: req.Description,
		DueDate: service.OptionalTime{
			Set:   req.DueDate.Set,
			Value: req.DueDate.Value,
		},
	}
}

// ListInput maps a list query to service input.
func ListInput(req request.ListTodos) service.ListInput {
	return service.ListInput{
		Page:      req.Page,
		PageSize:  req.PageSize,
		Completed: req.Completed,
		Query:     req.Query,
		Sort:      req.Sort,
		Order:     req.Order,
	}
}

// Todo maps a persistence model to a response DTO.
func Todo(item models.Todo) response.Todo {
	return response.Todo{
		ID:          item.ID.String(),
		Title:       item.Title,
		Description: item.Description,
		Completed:   item.Completed,
		DueDate:     item.DueDate,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

// TodoList maps a service list result to a response DTO.
func TodoList(result service.ListResult) response.TodoList {
	items := make([]response.Todo, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, Todo(item))
	}
	return response.TodoList{
		Data: items,
		Pagination: response.Pagination{
			Page:       result.Page,
			PageSize:   result.PageSize,
			Total:      result.Total,
			TotalPages: totalPages(result.Total, result.PageSize),
		},
	}
}

func totalPages(total int64, pageSize int) int {
	if total <= 0 || pageSize <= 0 {
		return 0
	}
	pages := (total + int64(pageSize) - 1) / int64(pageSize)
	return int(pages)
}
