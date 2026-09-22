package validator

import (
	"strings"
)

func IsValidLuhn(number string) bool {
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")

	if len(number) == 0 {
		return false
	}

	for _, char := range number {
		if char < '0' || char > '9' {
			return false
		}
	}

	var sum int
	parity := len(number) % 2

	for i, char := range number {

		digit := int(char - '0')

		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}

	return sum%10 == 0
}
