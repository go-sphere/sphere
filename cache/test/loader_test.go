package test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/memory"
)

func testGetObjectEx(ctx context.Context, t *testing.T, byteCache cache.ByteCache) {
	type Example struct {
		Value string `json:"value"`
	}
	val, _ := json.Marshal(Example{Value: "testValue"})

	_ = byteCache.Set(ctx, "testKey", val)

	type args[D codec.Decoder, E codec.Encoder, T any] struct {
		ctx     context.Context
		c       cache.ByteCache
		d       D
		e       E
		key     string
		builder func() (obj T, err error)
	}
	type testCase[D codec.Decoder, E codec.Encoder, T any] struct {
		name      string
		args      args[D, E, T]
		want      T
		wantFound bool
		wantErr   bool
	}
	tests := []testCase[codec.DecoderFunc, codec.EncoderFunc, *Example]{
		{
			name: "GetObjectEx existing key",
			args: args[codec.DecoderFunc, codec.EncoderFunc, *Example]{
				ctx:     ctx,
				c:       byteCache,
				d:       json.Unmarshal,
				e:       json.Marshal,
				key:     "testKey",
				builder: nil,
			},
			want:      &Example{Value: "testValue"},
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "GetObjectEx non-existing key with builder",
			args: args[codec.DecoderFunc, codec.EncoderFunc, *Example]{
				ctx: ctx,
				c:   byteCache,
				d:   json.Unmarshal,
				e:   json.Marshal,
				key: "testKeyNotFound",
				builder: func() (*Example, error) {
					return &Example{Value: "newValue"}, nil
				},
			},
			want:      &Example{Value: "newValue"},
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "GetObjectEx with error in builder",
			args: args[codec.DecoderFunc, codec.EncoderFunc, *Example]{
				ctx: ctx,
				c:   byteCache,
				d:   json.Unmarshal,
				e:   json.Marshal,
				key: "testKeyError",
				builder: func() (*Example, error) {
					return nil, errors.New("test error")
				},
			},
			want:      nil,
			wantFound: false,
			wantErr:   true,
		},
		{
			name: "GetObjectEx with nil builder",
			args: args[codec.DecoderFunc, codec.EncoderFunc, *Example]{
				ctx: ctx,
				c:   byteCache,
				d:   json.Unmarshal,
				e:   json.Marshal,
				key: "testKeyNilBuilder",
				builder: func() (*Example, error) {
					return nil, nil // Simulating a nil builder
				},
			},
			want:      nil,
			wantFound: true,
			wantErr:   false, // Expect no error when builder returns nil
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := cache.GetObjectEx(tt.args.ctx, tt.args.c, tt.args.d, tt.args.e, tt.args.key, tt.args.builder)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetEx() got = %v, want %v", got, tt.want)
			}
			if found != tt.wantFound {
				t.Errorf("GetEx() found = %v, want %v", found, tt.wantFound)
			}
		})
	}
}

func TestGetObjectEx(t *testing.T) {
	ctx := context.Background()
	byteCache := memory.NewByteCache()
	testGetObjectEx(ctx, t, byteCache)
}
