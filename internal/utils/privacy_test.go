package utils

import "testing"

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  string
	}{
		{name: "mainland mobile", phone: "13800138000", want: "138****8000"},
		{name: "trim whitespace", phone: " 13800138000 ", want: "138****8000"},
		{name: "short number", phone: "1234", want: "****"},
		{name: "empty", phone: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskPhone(tt.phone); got != tt.want {
				t.Errorf("MaskPhone(%q) = %q, want %q", tt.phone, got, tt.want)
			}
		})
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{email: "alice@example.invalid", want: "a****@example.invalid"},
		{email: "a@example.invalid", want: "a***@example.invalid"},
		{email: "", want: ""},
		{email: "invalid-address", want: "***************"},
	}
	for _, tt := range tests {
		if got := MaskEmail(tt.email); got != tt.want {
			t.Errorf("MaskEmail(%q) = %q, want %q", tt.email, got, tt.want)
		}
	}
}
