package localfs

import (
	"mime"
	"net/http"
	pathpkg "path"
)

func detectMimeType(relPath string, preview []byte) string {
	if mimeType := mime.TypeByExtension(pathpkg.Ext(relPath)); mimeType != "" {
		return mimeType
	}
	if len(preview) == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(preview)
}

func detectExportMimeType(relPath string) string {
	if mimeType := mime.TypeByExtension(pathpkg.Ext(relPath)); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}
