package exitcode

import (
	"errors"
	"testing"

	"github.com/axsh/entext/internal/common/apperr"
)

func TestFromError(t *testing.T) {
	t.Parallel()

	if got := FromError(nil); got != CodeOK {
		t.Fatalf("nil should be ok, got=%d", got)
	}
	if got := FromError(apperr.NewValidationError(apperr.ErrInvalidArgs)); got != CodeInvalidArgs {
		t.Fatalf("validation should map to invalid args, got=%d", got)
	}
	if got := FromError(errors.New("runtime")); got != CodeRuntimeErr {
		t.Fatalf("runtime should map to runtime err, got=%d", got)
	}
}
