package validation

// FieldError ties a failed rule to the field it was checked on.
type FieldError struct {
	Field string
	Err   error
}

// Errors collects failures across fields, in the order they were added, so a
// caller can report every problem at once.
type Errors []FieldError

// Add records err for field. A nil err is ignored, so calls can be chained
// directly onto a rule: errs.Add("title", validation.String(...)).
func (e *Errors) Add(field string, err error) {
	if err != nil {
		*e = append(*e, FieldError{Field: field, Err: err})
	}
}
