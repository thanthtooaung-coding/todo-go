// Package response defines todo HTTP response DTOs.
package response

import "time"

// Todo is a todo item returned to clients.
type Todo struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Completed   bool       `json:"completed"`
	DueDate     *time.Time `json:"due_date"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TodoList is a page of todo items.
type TodoList struct {
	Data       []Todo     `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// Pagination describes the current page.
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}
