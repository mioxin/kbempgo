package utils

import (
	"testing"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/datasource"

	"github.com/stretchr/testify/assert"
)

func TestExtractDigits(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "digit&alpha",
			input:    "abc123def456",
			expected: "123456",
		},
		{
			name:     "nonASCII",
			input:    "Привет, 2025-11-24! 😊 +7(777)123-45-67",
			expected: "2025112477771234567",
		},
		{
			name:     "non digit",
			input:    "no digits here! 😊",
			expected: "",
		},
		{
			name:     "digit",
			input:    "007007",
			expected: "007007",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractDigits(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvKbv2Phone(t *testing.T) {
	// Мок Sotr с Phone
	sotrWithPhone := &kbv1.Sotr{
		Phone: []string{"123-456", "999 00 11"},
	}
	sotrEmptyPhone := &kbv1.Sotr{
		Phone: []string{},
	}
	sotrNilPhone := &kbv1.Sotr{} // Phone по умолчанию nil-слайс

	tests := []struct {
		name     string
		input    *kbv1.Sotr
		expected []datasource.Phone
	}{
		{
			name:  "With Phone",
			input: sotrWithPhone,
			expected: []datasource.Phone{
				{Phone: "123-456"},
				{Phone: "999 00 11"},
			},
		},
		{
			name:     "empty Phone",
			input:    sotrEmptyPhone,
			expected: []datasource.Phone{},
		},
		{
			name:     "nil Phone",
			input:    sotrNilPhone,
			expected: []datasource.Phone{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvKbv2Phone(tt.input)
			assert.Equal(t, tt.expected, result)

			// Проверка, что строки скопированы (не ссылки)
			if len(tt.expected) > 0 {
				assert.NotSame(t, &tt.input.Phone[0], &result[0].Phone)
				assert.Equal(t, tt.input.Phone[0], result[0].Phone) // значение то же
			}
		})
	}
}

func TestConvKbv2Mobile(t *testing.T) {
	// Мок Sotr с Mobile
	sotrWithMobile := &kbv1.Sotr{
		Mobile: []string{"+7 (777) 123-45-67", "8(705)999-00-11"},
	}
	sotrEmptyMobile := &kbv1.Sotr{
		Mobile: []string{},
	}
	sotrNilMobile := &kbv1.Sotr{} // Mobile по умолчанию nil-слайс

	tests := []struct {
		name     string
		input    *kbv1.Sotr
		expected []datasource.Mobile
	}{
		{
			name:  "with Mobile",
			input: sotrWithMobile,
			expected: []datasource.Mobile{
				{Mobile: 77771234567},
				{Mobile: 87059990011},
			},
		},
		{
			name:     "empty Mobile",
			input:    sotrEmptyMobile,
			expected: []datasource.Mobile{},
		},
		{
			name:     "nil Mobile",
			input:    sotrNilMobile,
			expected: []datasource.Mobile{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Мок slog (чтобы не засорять вывод)
			// slog.SetDefault(testlogger.New(t)) // testify slog mock

			result := ConvKbv2Mobile(tt.input)
			assert.Equal(t, tt.expected, result)

		})
	}
}
