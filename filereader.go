package eulumies

import (
	"bufio"
	"errors"
	"log"
	"strconv"
	"strings"
)

func validateStringFromLine(scanner *bufio.Scanner, maxLength int, strict bool) (string, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		} else {
			return "", errors.New("unexpected EOF")
		}
	}
	cleanLine := strings.TrimSpace(scanner.Text())
	if len(cleanLine) > maxLength && strict {
		return "", errors.New("line exceeds maximum allowed length: " + cleanLine)
	} else if len(cleanLine) > maxLength && !strict {
		log.Printf("[W] line exceeds maximum allowed length: %d > %d, %s", len(cleanLine), maxLength, cleanLine)
	}
	return cleanLine, nil
}

func validateIntFromLine(scanner *bufio.Scanner) (int, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return -1, err
		} else {
			return -1, errors.New("unexpected EOF")
		}
	}

	cleanLine := strings.TrimSpace(scanner.Text())
	// also replace spaces and underscores
	cleanLine = strings.ReplaceAll(cleanLine, " ", "")
	cleanLine = strings.ReplaceAll(cleanLine, "_", "")

	if len(cleanLine) == 0 {
		return -1, errors.New("line contains no integer")
	}

	value, err := strconv.Atoi(cleanLine)

	return value, err
}

func validateFloatFromLine(scanner *bufio.Scanner) (float64, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return -1, err
		} else {
			return -1, errors.New("unexpected EOF")
		}
	}

	cleanLine := strings.TrimSpace(scanner.Text())
	// replace all commas if present with dots
	cleanLine = strings.ReplaceAll(cleanLine, ",", ".")
	// also replace spaces and underscores
	cleanLine = strings.ReplaceAll(cleanLine, " ", "")
	cleanLine = strings.ReplaceAll(cleanLine, "_", "")

	if len(cleanLine) == 0 {
		return -1, errors.New("line contains no float")
	}

	value, err := strconv.ParseFloat(cleanLine, 64)

	return value, err
}
