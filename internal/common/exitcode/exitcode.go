package exitcode

import (
	"os"

	"github.com/axsh/entext/internal/common/apperr"
)

const (
	CodeOK          = 0
	CodeRuntimeErr  = 1
	CodeInvalidArgs = 2
)

func FromError(err error) int {
	if err == nil {
		return CodeOK
	}
	if apperr.IsValidationError(err) {
		return CodeInvalidArgs
	}
	return CodeRuntimeErr
}

func ExitWithError(err error) {
	os.Exit(FromError(err))
}
