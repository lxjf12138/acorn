package path

import (
	"errors"
	pathpkg "path"
	"strings"
)

var ErrInvalidPath = errors.New("invalid workspace path")

func NormalizeWorkspacePath(input string, allowRoot bool) (string, error) {
	if strings.ContainsRune(input, 0) || strings.Contains(input, `\`) || looksLikeWindowsDrive(input) {
		return "", ErrInvalidPath
	}
	if input == "" || input == "." {
		if allowRoot {
			return "", nil
		}
		return "", ErrInvalidPath
	}
	if strings.HasPrefix(input, "/") {
		return "", ErrInvalidPath
	}
	for _, segment := range strings.Split(input, "/") {
		if segment == ".." {
			return "", ErrInvalidPath
		}
	}
	cleaned := pathpkg.Clean(input)
	if cleaned == "." {
		if allowRoot {
			return "", nil
		}
		return "", ErrInvalidPath
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", ErrInvalidPath
	}
	return cleaned, nil
}

func looksLikeWindowsDrive(input string) bool {
	return len(input) >= 2 && input[1] == ':' && ((input[0] >= 'a' && input[0] <= 'z') || (input[0] >= 'A' && input[0] <= 'Z'))
}
