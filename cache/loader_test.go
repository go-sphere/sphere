package cache

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/TBXark/sphere/cache/mcache"
	"reflect"
	"testing"
)

func TestGetObjectEx(t *testing.T) {
	type Example struct {
		Value string `json:"value"`
	}
	val, _ := json.Marshal(Example{Value: "testValue"})

	cache := mcache.NewMapCache[[]byte]()
	_ = cache.Set(context.Background(), "testKey", val)

	type args[D Decoder, E Encoder, T any] struct {
		ctx     context.Context
		c       ByteCache
		d       D
		e       E
		key     string
		builder func() (obj *T, err error)
	}
	type testCase[D Decoder, E Encoder, T any] struct {
		name      string
		args      args[D, E, T]
		want      T
		wantFound bool
		wantErr   bool
	}
	tests := []testCase[DecoderFunc, EncoderFunc, Example]{
		{
			name: "GetObjectEx existing key",
			args: args[DecoderFunc, EncoderFunc, Example]{
				ctx:     context.Background(),
				c:       cache,
				d:       json.Unmarshal,
				e:       json.Marshal,
				key:     "testKey",
				builder: nil,
			},
			want:      Example{Value: "testValue"},
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "GetObjectEx non-existing key with builder",
			args: args[DecoderFunc, EncoderFunc, Example]{
				ctx: context.Background(),
				c:   cache,
				d:   json.Unmarshal,
				e:   json.Marshal,
				key: "testKeyNotFound",
				builder: func() (*Example, error) {
					return &Example{Value: "newValue"}, nil
				},
			},
			want:      Example{Value: "newValue"},
			wantFound: true,
			wantErr:   false,
		},
		{
			name: "GetObjectEx with error in builder",
			args: args[DecoderFunc, EncoderFunc, Example]{
				ctx: context.Background(),
				c:   cache,
				d:   json.Unmarshal,
				e:   json.Marshal,
				key: "testKeyError",
				builder: func() (*Example, error) {
					return nil, errors.New("test error")
				},
			},
			want:      Example{},
			wantFound: false,
			wantErr:   true,
		},
		{
			name: "GetObjectEx with nil builder",
			args: args[DecoderFunc, EncoderFunc, Example]{
				ctx: context.Background(),
				c:   cache,
				d:   json.Unmarshal,
				e:   json.Marshal,
				key: "testKeyNilBuilder",
				builder: func() (*Example, error) {
					return nil, nil // Simulating a nil builder
				},
			},
			want:      Example{},
			wantFound: false,
			wantErr:   false, // Expect no error when builder returns nil
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := GetObjectEx(tt.args.ctx, tt.args.c, tt.args.d, tt.args.e, tt.args.key, tt.args.builder)
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
