package apperror

import (
	"errors"
	"testing"
)

func TestWrapKind_StatusCodeUsesTypedError(t *testing.T) {
	err := WrapKind(ErrValidation, "orders.create", errors.New("invalid payload"))

	if got := StatusCode(err); got != 400 {
		t.Fatalf("StatusCode() = %d, want 400", got)
	}
}

func TestStack_WrapIncludesFrames(t *testing.T) {
	err := Wrap("orders.update", errors.New("boom"))
	appErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("Wrap() should return *Error")
	}

	stack := appErr.Stack()
	if len(stack) == 0 {
		t.Fatalf("Stack() should include at least one frame")
	}
}
