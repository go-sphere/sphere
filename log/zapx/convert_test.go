package zapx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"reflect"
	"strings"
	"sync"
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
	backend := newBackendFromCore(core)

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

type cyclicSlogGroup struct {
	branch bool
}

func (c cyclicSlogGroup) LogValue() slog.Value {
	if c.branch {
		return slog.GroupValue(slog.Any("a", c), slog.Any("b", c))
	}
	return slog.GroupValue(slog.Any("self", c))
}

func TestAttrToZapFieldBoundsCyclicGroupsAndReusesSafely(t *testing.T) {
	f := AttrToZapField(corelog.Any("g", cyclicSlogGroup{}))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			enc := zapcore.NewMapObjectEncoder()
			f.AddTo(enc)
			if _, ok := enc.Fields["g"]; !ok {
				t.Errorf("missing g field: %#v", enc.Fields)
			}
		})
	}
	wg.Wait()

	enc := zapcore.NewMapObjectEncoder()
	AttrToZapField(corelog.Any("branch", cyclicSlogGroup{branch: true})).AddTo(enc)
	if _, ok := enc.Fields["branch"]; !ok {
		t.Fatalf("missing branch field: %#v", enc.Fields)
	}
}

type cyclicInlineSlogGroup struct{}

func (c cyclicInlineSlogGroup) LogValue() slog.Value {
	return slog.GroupValue(slog.Any("", c))
}

func TestAttrToZapFieldEmptyKeyAndGroups(t *testing.T) {
	t.Run("skips empty-key non-group and empty groups", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			attr corelog.Attr
		}{
			{"zero attr", slog.Attr{}},
			{"empty key string", corelog.String("", "ignored")},
			{"empty group", corelog.Group("empty")},
			{"group with zero attr", corelog.Group("empty", slog.Attr{})},
			{"group with empty subgroup", corelog.Group("empty", corelog.Group("sub"))},
			{"empty inline group", corelog.Group("", slog.Attr{})},
		} {
			t.Run(tc.name, func(t *testing.T) {
				f := AttrToZapField(tc.attr)
				if f.Type != zapcore.SkipType {
					t.Fatalf("AttrToZapField(%s).Type = %v, want SkipType", tc.name, f.Type)
				}
				enc := zapcore.NewMapObjectEncoder()
				f.AddTo(enc)
				if len(enc.Fields) != 0 {
					t.Fatalf("encoded fields = %#v, want empty", enc.Fields)
				}
			})
		}
	})

	t.Run("inlines empty-key group at top level and inside named group", func(t *testing.T) {
		enc := zapcore.NewMapObjectEncoder()
		AttrToZapField(corelog.Group("",
			corelog.String("a", "1"),
			corelog.Int("b", 2),
			slog.Attr{},
			corelog.Group("empty"),
		)).AddTo(enc)
		wantTop := map[string]any{"a": "1", "b": int64(2)}
		if !reflect.DeepEqual(enc.Fields, wantTop) {
			t.Fatalf("top-level inline fields = %#v, want %#v", enc.Fields, wantTop)
		}

		nestedEnc := zapcore.NewMapObjectEncoder()
		AttrToZapField(corelog.Group("outer",
			slog.Attr{},
			corelog.Group("", corelog.String("k", "v")),
			corelog.Group("empty"),
		)).AddTo(nestedEnc)
		wantNested := map[string]any{"outer": map[string]any{"k": "v"}}
		if !reflect.DeepEqual(nestedEnc.Fields, wantNested) {
			t.Fatalf("nested inline fields = %#v, want %#v", nestedEnc.Fields, wantNested)
		}
	})

	t.Run("bounds cyclic inline group with attr_error key", func(t *testing.T) {
		enc := zapcore.NewMapObjectEncoder()
		AttrToZapField(corelog.Any("", cyclicInlineSlogGroup{})).AddTo(enc)
		raw, err := json.Marshal(enc.Fields)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if !strings.Contains(string(raw), `"attr_error"`) || !strings.Contains(string(raw), "unformattable") {
			t.Fatalf("cyclic inline fields = %s, want attr_error with unformattable", raw)
		}
	})
}

func TestBackendLogNonFiniteFloatsAndEmptyKeyParity(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buf), zapcore.DebugLevel)
	backend := newBackendFromCore(core, corelog.WithAttrs(map[string]any{
		"":     "ignored_preset",
		"keep": "ok",
	}))

	backend.Log(context.Background(), corelog.LevelInfo, "parity",
		slog.Attr{},
		corelog.String("", "ignored"),
		corelog.Group("empty"),
		corelog.Group("", corelog.String("inlined", "yes")),
		corelog.Float64("nan", math.NaN()),
		corelog.Float64("pos_inf", math.Inf(1)),
		corelog.Float64("neg_inf", math.Inf(-1)),
	)

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", buf.String(), err)
	}
	if got["keep"] != "ok" || got["inlined"] != "yes" {
		t.Fatalf("missing expected fields in %#v", got)
	}
	if got["nan"] != "NaN" || got["pos_inf"] != "+Inf" || got["neg_inf"] != "-Inf" {
		t.Fatalf("non-finite float fields = %#v", got)
	}
	if _, exists := got[""]; exists {
		t.Fatalf("empty key leaked into %#v", got)
	}
	if _, exists := got["empty"]; exists {
		t.Fatalf("empty group leaked into %#v", got)
	}
}
