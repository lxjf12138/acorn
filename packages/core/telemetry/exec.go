package telemetry

import "strings"

func SafeCommandName(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	command = strings.ReplaceAll(command, "\\", "/")
	parts := strings.Split(command, "/")
	name := strings.TrimSpace(parts[len(parts)-1])
	if name == "." || name == ".." {
		return ""
	}
	return name
}
