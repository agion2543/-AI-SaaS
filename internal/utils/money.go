package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func FenToYuan(amount int64) string {
	return fmt.Sprintf("%.2f", float64(amount)/100)
}

func YuanToFen(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}

	negative := false
	if strings.HasPrefix(value, "-") {
		negative = true
		value = strings.TrimPrefix(value, "-")
	}

	parts := strings.SplitN(value, ".", 3)
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount: %s", value)
	}

	integerPart := parts[0]
	if integerPart == "" {
		integerPart = "0"
	}

	decimalPart := "00"
	if len(parts) == 2 {
		decimalPart = parts[1]
		if len(decimalPart) == 1 {
			decimalPart += "0"
		}
		if len(decimalPart) > 2 {
			decimalPart = decimalPart[:2]
		}
	}

	total, err := strconv.ParseInt(integerPart+decimalPart, 10, 64)
	if err != nil {
		return 0, err
	}
	if negative {
		total = -total
	}
	return total, nil
}
