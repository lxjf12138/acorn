package eventing

import (
	"context"
	"fmt"
	"time"

	"github.com/lxjf12138/acorn/packages/core/events"
	otellog "go.opentelemetry.io/otel/log"
	otelglobal "go.opentelemetry.io/otel/log/global"
)

type OTelLogEmitter struct {
	logger otellog.Logger
}

func NewOTelLogEmitter(loggerName string) *OTelLogEmitter {
	if loggerName == "" {
		loggerName = "acorn/events"
	}
	return &OTelLogEmitter{logger: otelglobal.Logger(loggerName)}
}

func (e *OTelLogEmitter) Emit(ctx context.Context, event events.Event) {
	if e == nil || e.logger == nil || event.Name == "" {
		return
	}
	record := otellog.Record{}
	timestamp := event.Time
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	record.SetTimestamp(timestamp)
	record.SetObservedTimestamp(time.Now())
	record.SetEventName(event.Name)
	record.SetSeverity(otelSeverity(event.Severity))
	record.SetSeverityText(string(event.Severity))
	record.AddAttributes(eventAttributes(event.Attributes)...)
	e.logger.Emit(ctx, record)
}

func otelSeverity(severity events.Severity) otellog.Severity {
	switch severity {
	case events.SeverityError:
		return otellog.SeverityError
	case events.SeverityWarning:
		return otellog.SeverityWarn
	case events.SeverityInfo, "":
		return otellog.SeverityInfo
	default:
		return otellog.SeverityInfo
	}
}

func eventAttributes(attrs map[string]any) []otellog.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	kvs := make([]otellog.KeyValue, 0, len(attrs))
	for key, value := range attrs {
		if key == "" || value == nil {
			continue
		}
		kvs = append(kvs, otellog.KeyValue{Key: key, Value: eventValue(value)})
	}
	return kvs
}

func eventValue(value any) otellog.Value {
	switch v := value.(type) {
	case string:
		return otellog.StringValue(v)
	case bool:
		return otellog.BoolValue(v)
	case int:
		return otellog.IntValue(v)
	case int32:
		return otellog.Int64Value(int64(v))
	case int64:
		return otellog.Int64Value(v)
	case uint:
		return otellog.Int64Value(int64(v))
	case uint32:
		return otellog.Int64Value(int64(v))
	case uint64:
		if v > uint64(^uint(0)>>1) {
			return otellog.StringValue(fmt.Sprint(v))
		}
		return otellog.Int64Value(int64(v))
	case float32:
		return otellog.Float64Value(float64(v))
	case float64:
		return otellog.Float64Value(v)
	default:
		return otellog.StringValue(fmt.Sprint(v))
	}
}
