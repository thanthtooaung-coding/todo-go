package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/handler"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/routes"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/service"
	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type fakeService struct {
	createFn        func(context.Context, service.CreateInput) (models.Todo, error)
	listFn          func(context.Context, service.ListInput) (service.ListResult, error)
	getFn           func(context.Context, uuid.UUID) (models.Todo, error)
	updateFn        func(context.Context, uuid.UUID, service.UpdateInput) (models.Todo, error)
	setCompletionFn func(context.Context, uuid.UUID, bool) (models.Todo, error)
	deleteFn        func(context.Context, uuid.UUID) error
}

func (f *fakeService) Create(ctx context.Context, in service.CreateInput) (models.Todo, error) {
	return f.createFn(ctx, in)
}

func (f *fakeService) List(ctx context.Context, in service.ListInput) (service.ListResult, error) {
	return f.listFn(ctx, in)
}

func (f *fakeService) Get(ctx context.Context, id uuid.UUID) (models.Todo, error) {
	return f.getFn(ctx, id)
}

func (f *fakeService) Update(ctx context.Context, id uuid.UUID, in service.UpdateInput) (models.Todo, error) {
	return f.updateFn(ctx, id, in)
}

func (f *fakeService) SetCompletion(ctx context.Context, id uuid.UUID, completed bool) (models.Todo, error) {
	return f.setCompletionFn(ctx, id, completed)
}

func (f *fakeService) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteFn(ctx, id)
}

func TestTodoHTTP(t *testing.T) {
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	item := models.Todo{
		ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Title:       "Buy milk",
		Description: "2%",
		Completed:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc := &fakeService{
		createFn: func(_ context.Context, in service.CreateInput) (models.Todo, error) {
			if in.Title != "Buy milk" {
				t.Fatalf("title %q", in.Title)
			}
			return item, nil
		},
		listFn: func(_ context.Context, in service.ListInput) (service.ListResult, error) {
			if in.Sort != models.SortTitle || in.Query != "milk" || in.Completed == nil || !*in.Completed {
				t.Fatalf("%+v", in)
			}
			return service.ListResult{Items: []models.Todo{item}, Total: 1, Page: in.Page, PageSize: in.PageSize}, nil
		},
		getFn: func(_ context.Context, id uuid.UUID) (models.Todo, error) {
			if id != item.ID {
				return models.Todo{}, apperrors.NotFound("todo not found")
			}
			return item, nil
		},
		updateFn: func(_ context.Context, id uuid.UUID, in service.UpdateInput) (models.Todo, error) {
			if in.DueDate.Set && in.DueDate.Value != nil {
				t.Fatal("expected due date to be cleared")
			}
			updated := item
			updated.Title = *in.Title
			return updated, nil
		},
		setCompletionFn: func(_ context.Context, id uuid.UUID, completed bool) (models.Todo, error) {
			updated := item
			updated.Completed = completed
			return updated, nil
		},
		deleteFn: func(_ context.Context, id uuid.UUID) error {
			if id != item.ID {
				return apperrors.NotFound("todo not found")
			}
			return nil
		},
	}
	router := gin.New()
	routes.Register(router.Group("/api/v1"), handler.New(svc))

	t.Run("create", func(t *testing.T) {
		rec := perform(router, http.MethodPost, "/api/v1/todos", `{"title":" Buy milk ","description":"2%"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Location") != "/api/v1/todos/"+item.ID.String() {
			t.Fatal(rec.Header().Get("Location"))
		}
		var body struct {
			Data struct {
				ID        string `json:"id"`
				Completed bool   `json:"completed"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Data.ID != item.ID.String() || body.Data.Completed {
			t.Fatalf("%+v", body.Data)
		}
	})

	t.Run("create validation", func(t *testing.T) {
		rec := perform(router, http.MethodPost, "/api/v1/todos", `{"description":"missing title"}`)
		assertError(t, rec, http.StatusBadRequest, "validation_error")
	})

	t.Run("unknown field", func(t *testing.T) {
		rec := perform(router, http.MethodPost, "/api/v1/todos", `{"title":"Buy milk","completed":true}`)
		assertError(t, rec, http.StatusBadRequest, "validation_error")
	})

	t.Run("list", func(t *testing.T) {
		rec := perform(router, http.MethodGet, "/api/v1/todos?completed=true&q=milk&sort=title&order=asc", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("bad sort", func(t *testing.T) {
		rec := perform(router, http.MethodGet, "/api/v1/todos?sort=id%3Bdrop", "")
		assertError(t, rec, http.StatusBadRequest, "validation_error")
	})

	t.Run("get", func(t *testing.T) {
		rec := perform(router, http.MethodGet, "/api/v1/todos/"+item.ID.String(), "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("bad id", func(t *testing.T) {
		rec := perform(router, http.MethodGet, "/api/v1/todos/not-a-uuid", "")
		assertError(t, rec, http.StatusBadRequest, "validation_error")
	})

	t.Run("missing", func(t *testing.T) {
		rec := perform(router, http.MethodGet, "/api/v1/todos/"+uuid.NewString(), "")
		assertError(t, rec, http.StatusNotFound, "not_found")
	})

	t.Run("update", func(t *testing.T) {
		rec := perform(router, http.MethodPatch, "/api/v1/todos/"+item.ID.String(), `{"title":"Read book","due_date":null}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("completion false", func(t *testing.T) {
		rec := perform(router, http.MethodPatch, "/api/v1/todos/"+item.ID.String()+"/completion", `{"completed":false}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"completed":false`)) {
			t.Fatal(rec.Body.String())
		}
	})

	t.Run("delete", func(t *testing.T) {
		rec := perform(router, http.MethodDelete, "/api/v1/todos/"+item.ID.String(), "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
}

func perform(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != code {
		t.Fatalf("code %s", body.Error.Code)
	}
}
