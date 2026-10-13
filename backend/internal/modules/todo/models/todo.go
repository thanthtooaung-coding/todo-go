// Package models contains the todo persistence model and shared field limits.
package models

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	// MaxTitleLength is the maximum title length in characters.
	MaxTitleLength = 200
	// MaxDescriptionLength is the maximum description length in characters.
	MaxDescriptionLength = 2000
	// MaxQueryLength is the maximum title search length in characters.
	MaxQueryLength = 200
	// DefaultPage is the first page.
	DefaultPage = 1
	// DefaultPageSize is used when a list request omits page_size.
	DefaultPageSize = 20
	// MaxPageSize is the largest accepted page size.
	MaxPageSize = 100
	// MaxPage limits how far a client can page.
	MaxPage = 10000

	// SortCreatedAt sorts by creation time.
	SortCreatedAt = "created_at"
	// SortDueDate sorts by due date.
	SortDueDate = "due_date"
	// SortTitle sorts by title.
	SortTitle = "title"

	// OrderAsc is ascending order.
	OrderAsc = "asc"
	// OrderDesc is descending order.
	OrderDesc = "desc"

	// DefaultSort is the list sort used when the client omits sort.
	DefaultSort = SortCreatedAt
	// DefaultOrder is the list order used when the client omits order.
	DefaultOrder = OrderDesc
)

// Todo is the persistence model for a todo item.
type Todo struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Title       string     `gorm:"type:varchar(200);not null"`
	Description string     `gorm:"type:text;not null;default:''"`
	Completed   bool       `gorm:"not null;default:false"`
	DueDate     *time.Time `gorm:"type:timestamptz"`
	CreatedAt   time.Time  `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time  `gorm:"type:timestamptz;not null"`
}

// TableName returns the todos table name.
func (Todo) TableName() string {
	return "todos"
}

// NormalizeTitle trims a title and returns a client-safe message when it is invalid.
func NormalizeTitle(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "title is required"
	}
	if strings.ContainsRune(value, '\x00') {
		return "", "title contains an invalid character"
	}
	if utf8.RuneCountInString(value) > MaxTitleLength {
		return "", "title must be at most 200 characters"
	}
	return value, ""
}

// NormalizeDescription trims a description and returns a client-safe message when it is invalid.
func NormalizeDescription(value string) (string, string) {
	value = strings.TrimSpace(value)
	if strings.ContainsRune(value, '\x00') {
		return "", "description contains an invalid character"
	}
	if utf8.RuneCountInString(value) > MaxDescriptionLength {
		return "", "description must be at most 2000 characters"
	}
	return value, ""
}

// NormalizeQuery trims a title search and returns a client-safe message when it is invalid.
func NormalizeQuery(value string) (string, string) {
	value = strings.TrimSpace(value)
	if strings.ContainsRune(value, '\x00') {
		return "", "q contains an invalid character"
	}
	if utf8.RuneCountInString(value) > MaxQueryLength {
		return "", "q must be at most 200 characters"
	}
	return value, ""
}
