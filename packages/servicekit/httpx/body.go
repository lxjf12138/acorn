package httpx

import (
	"net/http"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

func MaxBytesBody(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	if maxBytes <= 0 {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
}

func MaxBytesKratosBody(ctx khttp.Context, maxBytes int64) {
	if maxBytes <= 0 {
		return
	}
	MaxBytesBody(ctx.Response(), ctx.Request(), maxBytes)
}
