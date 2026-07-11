package exceltopdf

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/axsh/entext/internal/common/apperr"
)

func ParseSheetIndices(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	items := strings.Split(raw, ",")
	indices := make([]int, 0, len(items))
	seen := map[int]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		n, err := strconv.Atoi(item)
		if err != nil || n <= 0 {
			return nil, apperr.NewValidationError(fmt.Errorf("%w: invalid --sheets value: %s", apperr.ErrInvalidArgs, raw))
		}
		if _, ok := seen[n]; ok {
			return nil, apperr.NewValidationError(fmt.Errorf("%w: duplicate sheet index: %d", apperr.ErrInvalidArgs, n))
		}
		seen[n] = struct{}{}
		indices = append(indices, n)
	}
	return indices, nil
}
