package httpx

import (
	"io"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func ReadProtoJSON(ctx khttp.Context, msg proto.Message) error {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return err
	}
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(body, msg)
}

func WriteProtoJSON(ctx khttp.Context, code int, msg proto.Message) error {
	body, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}.Marshal(msg)
	if err != nil {
		return err
	}
	return ctx.Blob(code, "application/json", body)
}
