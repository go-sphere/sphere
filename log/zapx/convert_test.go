package zapx

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type objectMarshalingError struct{}

func (objectMarshalingError) Error() string { return "boom" }

func (objectMarshalingError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("detail", "object")
	return nil
}

func TestErrorFieldPreservesErrorEncoding(t *testing.T) {
	err := objectMarshalingError{}
	for _, tc := range []struct {
		name string
		attr corelog.Attr
		want any
	}{
		{"top level", corelog.Err(err), "boom"},
		{"grouped", corelog.Group("g", corelog.Err(err)), map[string]any{"error": "boom"}},
		{"custom key", corelog.Any("cause", err), map[string]any{"detail": "object"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enc := zapcore.NewMapObjectEncoder()
			AttrToZapField(tc.attr).AddTo(enc)
			if got := enc.Fields[tc.attr.Key]; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("encoded value = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestGroupAttrsKeepTypedEncoding pins that a group is encoded with the same
// typed encoders as a top-level attr. Flattening it into a map[string]any
// instead routed it through zap's reflection/JSON path, which renders a nested
// time.Duration as its nanosecond integer and errors as reflection output, so
// the same logical value changed shape with nesting depth.
func TestGroupAttrsKeepTypedEncoding(t *testing.T) {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeDuration = zapcore.StringDurationEncoder

	var buf bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encCfg), zapcore.AddSync(&buf), zapcore.DebugLevel)
	backend := newBackendWithLogger(zap.New(core), "")

	backend.Log(context.Background(), corelog.LevelInfo, "grouped",
		corelog.Duration("top", 5*time.Second),
		corelog.Group("g",
			corelog.Duration("d", 5*time.Second),
			corelog.Err(errors.New("boom")),
		),
	)

	out := buf.String()
	for _, want := range []string{`"top":"5s"`, `"d":"5s"`, `"error":"boom"`} {
		if !strings.Contains(out, want) {
			t.Errorf("output = %s, want it to contain %s", strings.TrimSpace(out), want)
		}
	}
}
