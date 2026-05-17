package httpx

import (
	"encoding/json"
	"net/http"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func HTTPStatusFromError(err error) int {
	switch status.Code(err) {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.FailedPrecondition, codes.Aborted:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusRequestEntityTooLarge
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func WriteError(ctx khttp.Context, statusCode int, code string, message string) error {
	if code == "" {
		code = "request_failed"
	}
	body, err := json.Marshal(ErrorResponse{
		Code:    code,
		Message: message,
	})
	if err != nil {
		return err
	}
	return ctx.Blob(statusCode, "application/json", body)
}

func ErrorEncoder(w http.ResponseWriter, _ *http.Request, err error) {
	code := status.Code(err).String()
	if code == "Unknown" {
		code = "request_failed"
	}
	message := err.Error()
	if st, ok := status.FromError(err); ok {
		message = st.Message()
	}
	body, marshalErr := json.Marshal(ErrorResponse{
		Code:    code,
		Message: message,
	})
	if marshalErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(HTTPStatusFromError(err))
	_, _ = w.Write(body)
}
