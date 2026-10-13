package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/repository"
	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
)

type fakeStore struct {
	items map[uuid.UUID]models.Todo
}

func newFakeStore() *fakeStore {
	return &fakeStore{items: map[uuid.UUID]models.Todo{}}
}

func (f *fakeStore) Create(_ context.Context, todo *models.Todo) error {
	if todo.ID == uuid.Nil {
		todo.ID = uuid.New()
	}
	now := time.Now().UTC()
	todo.CreatedAt = now
	todo.UpdatedAt = now
	f.items[todo.ID] = *todo
	return nil
}

func (f *fakeStore) GetByID(_ context.Context, id uuid.UUID) (models.Todo, error) {
	item, ok := f.items[id]
	if !ok {
		return models.Todo{}, apperrors.NotFound("todo not found")
	}
	return item, nil
}

func (f *fakeStore) List(_ context.Context, params repository.ListParams) (repository.ListResult, error) {
	items := make([]models.Todo, 0, len(f.items))
	for _, item := range f.items {
		items = append(items, item)
	}
	return repository.ListResult{Items: items, Total: int64(len(items))}, nil
}

func (f *fakeStore) Update(_ context.Context, todo *models.Todo) error {
	if _, ok := f.items[todo.ID]; !ok {
		return apperrors.NotFound("todo not found")
	}
	f.items[todo.ID] = *todo
	return nil
}

func (f *fakeStore) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.items[id]; !ok {
		return apperrors.NotFound("todo not found")
	}
	delete(f.items, id)
	return nil
}

func TestCreateTrimsTitleAndStaysIncomplete(t *testing.T) {
	store := newFakeStore()
	svc := New(store)
	due := time.Now().UTC().Add(time.Hour)
	item, err := svc.Create(context.Background(), CreateInput{Title: "  Buy milk  ", Description: " 2% ", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "Buy milk" || item.Description != "2%" || item.Completed || item.ID == uuid.Nil {
		t.Fatalf("%+v", item)
	}
}

func TestCreateRejectsBlankTitle(t *testing.T) {
	_, err := New(newFakeStore()).Create(context.Background(), CreateInput{Title: "   "})
	appErr := requireCode(t, err, apperrors.CodeValidation)
	if len(appErr.Details) == 0 || appErr.Details[0].Field != "title" {
		t.Fatal(appErr)
	}
}

func TestUpdateKeepsCompletionAndClearsDueDate(t *testing.T) {
	store := newFakeStore()
	svc := New(store)
	due := time.Now().UTC()
	created, err := svc.Create(context.Background(), CreateInput{Title: "Buy milk", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := svc.SetCompletion(context.Background(), created.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !completed.Completed {
		t.Fatal("expected completed")
	}
	title := "Read book"
	updated, err := svc.Update(context.Background(), created.ID, UpdateInput{
		Title:   &title,
		DueDate: OptionalTime{Set: true, Value: nil},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Read book" || !updated.Completed || updated.DueDate != nil {
		t.Fatalf("%+v", updated)
	}
}

func TestUpdateRequiresAField(t *testing.T) {
	_, err := New(newFakeStore()).Update(context.Background(), uuid.New(), UpdateInput{})
	requireCode(t, err, apperrors.CodeValidation)
}

func TestNotFound(t *testing.T) {
	svc := New(newFakeStore())
	id := uuid.New()
	_, err := svc.Get(context.Background(), id)
	requireCode(t, err, apperrors.CodeNotFound)
	err = svc.Delete(context.Background(), id)
	requireCode(t, err, apperrors.CodeNotFound)
}

func TestListRejectsSortInjection(t *testing.T) {
	_, err := New(newFakeStore()).List(context.Background(), ListInput{
		Page:     1,
		PageSize: 20,
		Sort:     "created_at; drop table todos",
		Order:    models.OrderDesc,
	})
	requireCode(t, err, apperrors.CodeValidation)
}

func TestListDefaults(t *testing.T) {
	store := newFakeStore()
	svc := New(store)
	if _, err := svc.Create(context.Background(), CreateInput{Title: "Buy milk"}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.List(context.Background(), ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != 1 || result.PageSize != 20 || result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("%+v", result)
	}
}

func requireCode(t *testing.T, err error, code apperrors.Code) *apperrors.Error {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
	return appErr
}
