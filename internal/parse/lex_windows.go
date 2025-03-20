package parse

import (
	"os"
	"strings"
)

func Split(commandString string) ([]string, error) {
	tokens := make([]string, 0)
	argBuilder := strings.Builder{}
	inQuotes, escaped := false, false
	operators := []string{
		"&&", "||", ">>", "<<", // 2-char operators first
		"|", "&", ">", "<", "(", ")", // 1-char operators last
	}
	length := len(commandString)
	runes := []rune(commandString)
	i := 0

	for i < len(runes) {
		char := runes[i]

		if char == '\n' || char == '\r' {
			char = ' '
		}

		if char == '\'' {
			char = '"'
		}

		if !inQuotes && char == '^' && !escaped {
			escaped = true
			i++
			continue
		}

		if escaped {
			argBuilder.WriteRune(char)
			escaped = false
			i++
			continue
		}

		if char == '"' {
			inQuotes = !inQuotes
			i++
			continue
		}

		if char == '%' && !inQuotes {
			newIndex, err := handleEnvVar(commandString, &argBuilder, i)
			if err != nil {
				return nil, err
			}
			i = newIndex
			continue
		}

		if char == '\\' {
			newIndex := handleBackslashes(runes, &argBuilder, &inQuotes, i)
			i = newIndex
			continue
		}

		if !inQuotes && (char == ' ' || char == '\t') {
			if argBuilder.Len() > 0 {
				tokens = append(tokens, argBuilder.String())
				argBuilder.Reset()
			}
			i++
			continue
		}

		if !inQuotes && handleOperators(commandString, &tokens, &argBuilder, operators, length, &i) {
			i++
			continue
		}

		argBuilder.WriteRune(char)
		i++
	}

	if argBuilder.Len() > 0 {
		tokens = append(tokens, argBuilder.String())
	}

	return tokens, nil
}

func handleEnvVar(commandString string, argBuilder *strings.Builder, i int) (int, error) {
	end := i + 1
	varNameBuilder := strings.Builder{}

	for end < len(commandString) {
		rVar := rune(commandString[end])
		if rVar == '%' {
			break
		}
		varNameBuilder.WriteRune(rVar)
		end++
	}

	if end < len(commandString) && commandString[end] == '%' {
		varName := varNameBuilder.String()
		varValue := os.Getenv(varName)
		argBuilder.WriteString(varValue)
		return end + 1, nil
	}

	return i + 1, nil
}

func handleBackslashes(runes []rune, argBuilder *strings.Builder, inQuotes *bool, i int) int {
	numBackslashes := 0
	for i < len(runes) && runes[i] == '\\' {
		numBackslashes++
		i++
	}

	if i < len(runes) && runes[i] == '"' {
		backslashesToAdd := numBackslashes / 2
		argBuilder.WriteString(strings.Repeat("\\", backslashesToAdd))
		if numBackslashes%2 == 0 {
			*inQuotes = !*inQuotes
		} else {
			argBuilder.WriteRune('"')
		}
		i++
	} else {
		argBuilder.WriteString(strings.Repeat("\\", numBackslashes))
	}
	return i
}

func handleOperators(commandString string, tokens *[]string, argBuilder *strings.Builder, operators []string, length int, index *int) bool {
	if index == nil || *index >= length {
		return false
	}

	// Try to match the longest operator at current position
	for _, op := range operators {
		if *index+len(op) < length && commandString[*index:*index+len(op)] == op {
			if argBuilder.Len() > 0 {
				*tokens = append(*tokens, argBuilder.String())
				argBuilder.Reset()
			}
			*tokens = append(*tokens, op)
			*index += len(op)

			return true
		}
	}

	return false
}
