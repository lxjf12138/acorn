package httpx

import "strings"

func SafeFilename(name string, fallback string) string {
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, `\`, "_")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		name = strings.TrimSpace(fallback)
	}
	if name == "" || name == "." || name == ".." {
		return "download"
	}
	return name
}
