package server_test

import (
	"errors"
	"reflect"
	"testing"

	"example.com/todo/internal/server"
	"example.com/todo/internal/validation"
)

func TestNewValidationErrorReportsAllFieldsInOrder(t *testing.T) {
	var errs validation.Errors
	errs.Add("title", validation.ErrMinLength)
	errs.Add("note", nil) // passes, so it must not appear
	errs.Add("due_date", validation.ErrRequired)
	errs.Add("other", errors.New("something unknown"))

	got := server.NewValidationError(errs)

	if got.Status != 400 || got.Code != "VALIDATION_FAILED" {
		t.Fatalf("got status %d code %q", got.Status, got.Code)
	}
	want := []server.FieldError{
		{Field: "title", Code: "MIN_LENGTH", Message: "min length"},
		{Field: "due_date", Code: "REQUIRED", Message: "required"},
		{Field: "other", Code: "INVALID", Message: "invalid"},
	}
	if !reflect.DeepEqual(got.Details, want) {
		t.Fatalf("details = %+v, want %+v", got.Details, want)
	}
}

func TestNewValidationErrorDoesNotModifySharedValue(t *testing.T) {
	var errs validation.Errors
	errs.Add("title", validation.ErrRequired)

	server.NewValidationError(errs)

	if server.ErrValidation.Details != nil {
		t.Fatal("ErrValidation was modified")
	}
}
