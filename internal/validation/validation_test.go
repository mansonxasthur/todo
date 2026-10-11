package validation_test

import (
	"errors"
	"strings"
	"testing"

	"example.com/todo/internal/validation"
)

func TestString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", validation.ErrRequired},
		{"below minimum", "ab", validation.ErrMinLength},
		{"minimum", "abc", nil},
		{"maximum", strings.Repeat("a", 255), nil},
		{"above maximum", strings.Repeat("a", 256), validation.ErrMaxLength},
		{"counts characters not bytes", "日本語", nil},
		{"255 multibyte characters", strings.Repeat("日", 255), nil},
		{"256 multibyte characters", strings.Repeat("日", 256), validation.ErrMaxLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validation.String(tt.in, 3, 255); !errors.Is(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
