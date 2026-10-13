package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/thanthtooaung-coding/todo-go/backend/internal/modules/todo/models"
	apperrors "github.com/thanthtooaung-coding/todo-go/backend/internal/shared/errors"
)

func TestOrderClauseAllowlist(t *testing.T) {
	clause, err := orderClause(models.SortCreatedAt, models.OrderDesc)
	if err != nil {
		t.Fatal(err)
	}
	if clause != "created_at DESC NULLS LAST" {
		t.Fatal(clause)
	}
	if _, err := orderClause("created_at; drop table todos", models.OrderAsc); err == nil {
		t.Fatal("expected sort to be rejected")
	}
	if _, err := orderClause(models.SortTitle, "desc; delete"); err == nil {
		t.Fatal("expected order to be rejected")
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`100%_a\b`); got != `100\%\_a\\b` {
		t.Fatal(got)
	}
}

func TestMapDBError(t *testing.T) {
	notFound := mapDBError(gorm.ErrRecordNotFound, "get todo")
	appErr, ok := errorsAs(notFound)
	if !ok || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("%v", notFound)
	}

	conflict := mapDBError(&pgconn.PgError{Code: "23505"}, "create todo")
	appErr, ok = errorsAs(conflict)
	if !ok || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("%v", conflict)
	}

	check := mapDBError(&pgconn.PgError{Code: "23514", ConstraintName: "todos_title_not_blank"}, "create todo")
	appErr, ok = errorsAs(check)
	if !ok || appErr.Code != apperrors.CodeValidation || len(appErr.Details) != 1 || appErr.Details[0].Field != "title" {
		t.Fatalf("%v", check)
	}
}

func errorsAs(err error) (*apperrors.Error, bool) {
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return nil, false
	}
	return appErr, true
}
