package apperr

import "errors"

var (
	ErrInvalidArgs   = errors.New("invalid arguments")
	ErrInputRequired = errors.New("input is required")
)

type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string {
	if e == nil || e.Err == nil {
		return ErrInvalidArgs.Error()
	}
	return e.Err.Error()
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewValidationError(err error) error {
	if err == nil {
		err = ErrInvalidArgs
	}
	return &ValidationError{Err: err}
}

func IsValidationError(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}
