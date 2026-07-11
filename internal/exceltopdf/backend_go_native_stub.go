//go:build !windows

package exceltopdf

import (
	"context"
	"errors"

	"github.com/axsh/entext/internal/common/sheetmap"
)

type GoNativeBackend struct{}

func NewGoNativeBackend() *GoNativeBackend {
	return &GoNativeBackend{}
}

func (b *GoNativeBackend) Convert(_ context.Context, _ string, _ string, _ []int) (string, sheetmap.SheetMap, error) {
	return "", sheetmap.SheetMap{}, errors.New("go-native backend requires windows with microsoft excel")
}
