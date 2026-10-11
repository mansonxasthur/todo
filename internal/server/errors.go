package server

import (
	"errors"
	"net/http"

	"example.com/todo/internal/validation"
)

// APIError bundles everything a client sees for one kind of failure.
// Codes are part of the public API: clients may branch on them, so never
// rename one once it ships. Messages are written for clients, so they must
// never be built from a raw error of a dependency. Details lists the
// individual problems of a validation failure.
type APIError struct {
	Status  int          `json:"-"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// FieldError describes one invalid field.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	ErrInvalidBody = APIError{Status: http.StatusBadRequest, Code: "INVALID_BODY", Message: "invalid request body"}
	ErrResourceID  = APIError{Status: http.StatusBadRequest, Code: "INVALID_RESOURCE_ID", Message: "invalid resource id"}
	ErrNotFound    = APIError{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: "resource not found"}
	ErrValidation  = APIError{Status: http.StatusBadRequest, Code: "VALIDATION_FAILED", Message: "validation failed"}
	ErrInternal    = APIError{Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR", Message: "internal server error"}
)

// Generic, field-agnostic codes used in the details of ErrValidation.
var (
	fieldRequired  = FieldError{Code: "REQUIRED", Message: "required"}
	fieldMinLength = FieldError{Code: "MIN_LENGTH", Message: "min length"}
	fieldMaxLength = FieldError{Code: "MAX_LENGTH", Message: "max length"}
	fieldInvalid   = FieldError{Code: "INVALID", Message: "invalid"}
)

// NewValidationError builds the response for failed validation, with one
// detail per failed field in the order they were added.
func NewValidationError(errs validation.Errors) APIError {
	e := ErrValidation
	e.Details = make([]FieldError, 0, len(errs))

	for _, fe := range errs {
		d := fieldDetail(fe.Err)
		d.Field = fe.Field
		e.Details = append(e.Details, d)
	}
	return e
}

func fieldDetail(err error) FieldError {
	switch {
	case errors.Is(err, validation.ErrRequired):
		return fieldRequired
	case errors.Is(err, validation.ErrMinLength):
		return fieldMinLength
	case errors.Is(err, validation.ErrMaxLength):
		return fieldMaxLength
	}
	return fieldInvalid
}
