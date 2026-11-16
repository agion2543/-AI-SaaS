package utils

import "strings"

// MaskPhone keeps only a short prefix and suffix for display in list views.
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	runes := []rune(phone)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) == 11 {
		return string(runes[:3]) + "****" + string(runes[7:])
	}
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:2]) + strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-2:])
}

// MaskEmail preserves the domain for account identification while hiding most
// of the local part. Values without a valid local/domain separator are hidden.
func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		if email == "" {
			return ""
		}
		return strings.Repeat("*", len([]rune(email)))
	}
	local := []rune(parts[0])
	if len(local) == 1 {
		return string(local[0]) + "***@" + parts[1]
	}
	return string(local[0]) + strings.Repeat("*", len(local)-1) + "@" + parts[1]
}
