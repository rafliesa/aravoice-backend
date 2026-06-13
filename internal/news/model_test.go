package news

import (
	"errors"
	"testing"
)

func TestInvalidInputError(t *testing.T) {
	err := &InvalidInputError{Message: "title is required"}

	if err.Error() != "title is required" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
	if !errors.Is(err, ErrInvalidNewsInput) {
		t.Fatal("expected InvalidInputError to wrap ErrInvalidNewsInput")
	}
}
