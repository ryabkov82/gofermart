package utils

import "strconv"

// ValidateOrderNumber проверяет номер заказа с помощью алгоритма Луна
func ValidateOrderNumber(number string) bool {

	sum := 0
	parity := len(number) % 2

	for i, digitChar := range number {
		digit, err := strconv.Atoi(string(digitChar))
		if err != nil {
			return false
		}

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
