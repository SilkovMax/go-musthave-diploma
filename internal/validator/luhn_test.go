package validator

import "testing"

func TestIsValidLuhn(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		expected bool
	}{
		{"Valid number 1", "9278923470", true},
		{"Valid number 2", "12345678903", true},
		{"Valid number with spaces", "927 892 347 0", true},
		{"Valid number with dashes", "927-892-347-0", true},
		{"Invalid number", "12345678904", false},
		{"Invalid characters", "12345abc", false},
		{"Empty string", "", false},
		{"Single zero", "0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidLuhn(tt.number)
			if result != tt.expected {
				t.Errorf("IsValidLuhn(%q) = %v, expected %v", tt.number, result, tt.expected)
			}
		})
	}
}
