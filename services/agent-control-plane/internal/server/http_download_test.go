package server

import (
	"context"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDownloadResourceStreamsBytesAndHeaders(t *testing.T) {
	store := resourcedomain.NewMemoryStore()
	if _, err := store.Register(context.Background(), &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			Id:                 "res_1",
			AuthorityServiceId: "sandbox-service",
			Name:               "reports/final\r\n.txt",
			MimeType:           "text/plain",
			SizeBytes:          11,
			ContentHash:        "sha256:abc",
		},
		OwnerUserId: "user-1",
		Visibility:  resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	gateway := service.NewResourceGatewayService(store, map[string]service.ResourceAuthorityClient{
		"sandbox-service": &downloadAuthorityClient{stream: &downloadStream{
			chunks: []*resourcev1.OpenResourceResponse{
				{
					Resource: &resourcev1.ResourceRef{
						Id:                 "res_1",
						AuthorityServiceId: "sandbox-service",
						Name:               "reports/final\r\n.txt",
						MimeType:           "text/plain",
						SizeBytes:          11,
						ContentHash:        "sha256:abc",
					},
					Data: []byte("hello "),
				},
				{Data: []byte("world")},
			},
		}},
	})
	recorder := httptest.NewRecorder()
	ctx := newDownloadTestContext("res_1", "user-1", recorder)

	err := downloadResource(ctx, gateway)
	if err != nil {
		t.Fatalf("downloadResource returned error: %v", err)
	}
	if recorder.Body.String() != "hello world" {
		t.Fatalf("unexpected body: %q", recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("unexpected content type: %q", got)
	}
	if got := recorder.Header().Get("Content-Length"); got != "11" {
		t.Fatalf("unexpected content length: %q", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); got != `attachment; filename="reports_final.txt"` {
		t.Fatalf("unexpected content disposition: %q", got)
	}
	if got := recorder.Header().Get("X-Acorn-Resource-ID"); got != "res_1" {
		t.Fatalf("unexpected resource id header: %q", got)
	}
	if got := recorder.Header().Get("X-Acorn-Content-Hash"); got != "sha256:abc" {
		t.Fatalf("unexpected content hash header: %q", got)
	}
}

func TestDownloadResourceRejectsOwnerMismatch(t *testing.T) {
	store := resourcedomain.NewMemoryStore()
	if _, err := store.Register(context.Background(), &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			Id:                 "res_1",
			AuthorityServiceId: "sandbox-service",
			Name:               "report.txt",
		},
		OwnerUserId: "user-1",
		Visibility:  resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	gateway := service.NewResourceGatewayService(store, map[string]service.ResourceAuthorityClient{"sandbox-service": &downloadAuthorityClient{}})
	err := downloadResource(newDownloadTestContext("res_1", "user-2", httptest.NewRecorder()), gateway)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestSafeDownloadFilename(t *testing.T) {
	tests := []struct {
		name     string
		fallback string
		want     string
	}{
		{name: " report.txt ", fallback: "res_1", want: "report.txt"},
		{name: "reports/final\\v1.txt", fallback: "res_1", want: "reports_final_v1.txt"},
		{name: "bad\r\nname.txt", fallback: "res_1", want: "badname.txt"},
		{name: " \r\n ", fallback: "res_1", want: "res_1"},
	}
	for _, tt := range tests {
		if got := safeDownloadFilename(tt.name, tt.fallback); got != tt.want {
			t.Fatalf("safeDownloadFilename(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

type downloadAuthorityClient struct {
	stream resourcev1.ResourceContentService_OpenResourceClient
	err    error
}

func (c *downloadAuthorityClient) OpenResource(context.Context, string) (resourcev1.ResourceContentService_OpenResourceClient, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.stream, nil
}

type downloadStream struct {
	resourcev1.ResourceContentService_OpenResourceClient
	chunks []*resourcev1.OpenResourceResponse
	index  int
}

func (s *downloadStream) Recv() (*resourcev1.OpenResourceResponse, error) {
	if s.index >= len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

type downloadTestContext struct {
	context.Context
	vars     url.Values
	query    url.Values
	request  *nethttp.Request
	response nethttp.ResponseWriter
}

func newDownloadTestContext(resourceID string, userID string, response nethttp.ResponseWriter) *downloadTestContext {
	req := httptest.NewRequest(nethttp.MethodGet, "/resources/"+resourceID+"/download?user_id="+url.QueryEscape(userID), nil)
	return &downloadTestContext{
		Context:  req.Context(),
		vars:     url.Values{"resource_id": []string{resourceID}},
		query:    req.URL.Query(),
		request:  req,
		response: response,
	}
}

func (c *downloadTestContext) Vars() url.Values                 { return c.vars }
func (c *downloadTestContext) Query() url.Values                { return c.query }
func (c *downloadTestContext) Form() url.Values                 { return nil }
func (c *downloadTestContext) Header() nethttp.Header           { return c.request.Header }
func (c *downloadTestContext) Request() *nethttp.Request        { return c.request }
func (c *downloadTestContext) Response() nethttp.ResponseWriter { return c.response }
func (c *downloadTestContext) Middleware(handler middleware.Handler) middleware.Handler {
	return handler
}
func (c *downloadTestContext) Bind(any) error                                 { return nil }
func (c *downloadTestContext) BindVars(any) error                             { return nil }
func (c *downloadTestContext) BindQuery(any) error                            { return nil }
func (c *downloadTestContext) BindForm(any) error                             { return nil }
func (c *downloadTestContext) Returns(any, error) error                       { return nil }
func (c *downloadTestContext) Result(int, any) error                          { return nil }
func (c *downloadTestContext) JSON(int, any) error                            { return nil }
func (c *downloadTestContext) XML(int, any) error                             { return nil }
func (c *downloadTestContext) String(int, string) error                       { return nil }
func (c *downloadTestContext) Blob(int, string, []byte) error                 { return nil }
func (c *downloadTestContext) Stream(int, string, io.Reader) error            { return nil }
func (c *downloadTestContext) Reset(nethttp.ResponseWriter, *nethttp.Request) {}

var _ khttp.Context = (*downloadTestContext)(nil)
