package request

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestCreateTodoDecodeAndValidate(t *testing.T) {
	var req CreateTodo
	body := []byte(`{"title":"  Buy milk ","description":"2%","due_date":"2026-10-15T09:00:00Z"}`)
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	if req.Title != "Buy milk" || req.Description != "2%" || req.DueDate == nil {
		t.Fatalf("%+v", req)
	}

	var rejected CreateTodo
	err := json.Unmarshal([]byte(`{"title":"Buy milk","completed":true}`), &rejected)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestUpdateTodoClearsDueDate(t *testing.T) {
	var req UpdateTodo
	if err := json.Unmarshal([]byte(`{"due_date":null}`), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	if !req.DueDate.Set || req.DueDate.Value != nil {
		t.Fatalf("%+v", req.DueDate)
	}

	var empty UpdateTodo
	if err := json.Unmarshal([]byte(`{}`), &empty); err != nil {
		t.Fatal(err)
	}
	if err := empty.Validate(); err == nil {
		t.Fatal("expected missing field error")
	}
}

func TestSetCompletionFalse(t *testing.T) {
	var req SetCompletion
	if err := json.Unmarshal([]byte(`{"completed":false}`), &req); err != nil {
		t.Fatal(err)
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	if req.Completed == nil || *req.Completed {
		t.Fatalf("%v", req.Completed)
	}
}

func TestParseListTodos(t *testing.T) {
	query, err := ParseListTodos(url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if query.Page != 1 || query.PageSize != 20 || query.Sort != "created_at" || query.Order != "desc" {
		t.Fatalf("%+v", query)
	}

	values := url.Values{}
	values.Set("page", "2")
	values.Set("page_size", "10")
	values.Set("completed", "true")
	values.Set("q", " milk ")
	values.Set("sort", "title")
	values.Set("order", "asc")
	query, err = ParseListTodos(values)
	if err != nil {
		t.Fatal(err)
	}
	if query.Page != 2 || query.PageSize != 10 || query.Completed == nil || !*query.Completed || query.Query != "milk" || query.Sort != "title" || query.Order != "asc" {
		t.Fatalf("%+v", query)
	}

	values.Set("sort", "id;drop")
	if _, err := ParseListTodos(values); err == nil {
		t.Fatal("expected sort error")
	}
}

func TestDueDateMustBeRFC3339(t *testing.T) {
	var req CreateTodo
	err := json.Unmarshal([]byte(`{"title":"Buy milk","due_date":"tomorrow"}`), &req)
	if err == nil {
		t.Fatal("expected due date error")
	}
	parsed := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	body, err := json.Marshal(map[string]any{"title": "Buy milk", "due_date": parsed.Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
}
