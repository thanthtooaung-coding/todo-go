// Package routes registers the todo HTTP endpoints.
package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/handler"
)

// Register attaches todo routes to the API group.
func Register(rg *gin.RouterGroup, h *handler.Handler) {
	rg.POST("/todos", h.Create)
	rg.GET("/todos", h.List)
	rg.GET("/todos/:id", h.Get)
	rg.PATCH("/todos/:id", h.Update)
	rg.PATCH("/todos/:id/completion", h.SetCompletion)
	rg.DELETE("/todos/:id", h.Delete)
}
