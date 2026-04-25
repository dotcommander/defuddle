package defuddle

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotHTML", ErrNotHTML},
		{"ErrTooLarge", ErrTooLarge},
		{"ErrTimeout", ErrTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Wrap the error
			wrapped := fmt.Errorf("context: %w", tt.err)
			assert.True(t, errors.Is(wrapped, tt.err))
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}
