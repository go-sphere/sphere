package cache

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/TBXark/sphere/cache/mcache"
	"golang.org/x/sync/singleflight"
)

func ptr[T any](v T) *T {
	return &v
}

func TestGet(t *testing.T) {
	cache := mcache.NewMapCache[string]()
	_ = cache.Set(context.Background(), "testKey", "testValue")

	type args[T any] struct {
		ctx        context.Context
		c          Cache[T]
		key        string
		expiration time.Duration
	}

	type testCase[T any] struct {
		name    string
		args    args[T]
		want    *T
		wantErr bool
	}

	tests := []testCase[string]{
		{
			name: "GetEx existing key",
			args: args[string]{
				ctx:        context.Background(),
				c:          cache,
				key:        "testKey",
				expiration: time.Minute,
			},
			want:    ptr("testValue"),
			wantErr: false,
		},
		{
			name: "GetEx non-existing key",
			args: args[string]{
				ctx:        context.Background(),
				c:          cache,
				key:        "testKeyNotFound",
				expiration: time.Minute,
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Get(tt.args.ctx, tt.args.c, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetX(t *testing.T) {
	cache := mcache.NewMapCache[string]()
	_ = cache.Set(context.Background(), "testKey", "testValue")

	type args[T any] struct {
		ctx context.Context
		c   Cache[T]
		key string
	}
	type testCase[T any] struct {
		name  string
		args  args[T]
		want  T
		want1 bool
	}
	tests := []testCase[string]{
		{
			name: "GetX existing key",
			args: args[string]{
				ctx: context.Background(),
				c:   cache,
				key: "testKey",
			},
			want:  "testValue",
			want1: true,
		},
		{
			name: "GetX non-existing key",
			args: args[string]{
				ctx: context.Background(),
				c:   cache,
				key: "testKeyNotFound",
			},
			want:  "",
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetX(tt.args.ctx, tt.args.c, tt.args.key)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetX() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetX() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetEx(t *testing.T) {
	cache := mcache.NewMapCache[string]()
	_ = cache.Set(context.Background(), "testKey", "testValue")

	type args[T any] struct {
		ctx        context.Context
		c          Cache[T]
		sf         *singleflight.Group
		key        string
		expiration time.Duration
		builder    func() (obj *T, err error)
	}
	type testCase[T any] struct {
		name    string
		args    args[T]
		want    *T
		wantErr bool
	}

	tests := []testCase[string]{
		{
			name: "GetEx existing key",
			args: args[string]{
				ctx:        context.Background(),
				c:          cache,
				sf:         &singleflight.Group{},
				key:        "testKey",
				expiration: time.Minute,
				builder:    nil,
			},
			want:    ptr("testValue"),
			wantErr: false,
		},
		{
			name: "GetEx non-existing key",
			args: args[string]{
				ctx:        context.Background(),
				c:          cache,
				sf:         &singleflight.Group{},
				key:        "testKeyNotFound",
				expiration: time.Minute,
				builder: func() (*string, error) {
					return ptr("newValue"), nil
				},
			},
			want:    ptr("newValue"),
			wantErr: false,
		},
		{
			name: "GetEx with error in builder",
			args: args[string]{
				ctx:        context.Background(),
				c:          cache,
				sf:         &singleflight.Group{},
				key:        "testKeyError",
				expiration: time.Minute,
				builder: func() (*string, error) {
					return nil, errors.New("test error")
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "GetEx with nil builder",
			args: args[string]{
				ctx:        context.Background(),
				c:          cache,
				sf:         &singleflight.Group{},
				key:        "testKeyNilBuilder",
				expiration: time.Minute,
				builder: func() (*string, error) {
					return nil, nil // Simulating a nil builder
				},
			},
			want:    nil,
			wantErr: false, // Expect no error when builder returns nil
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetEx(tt.args.ctx, tt.args.c, tt.args.sf, tt.args.key, tt.args.expiration, tt.args.builder)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetEx() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetObjectEx(t *testing.T) {
	type Example struct {
		Value string `json:"value"`
	}
	val, _ := json.Marshal(Example{Value: "testValue"})

	cache := mcache.NewMapCache[[]byte]()
	_ = cache.Set(context.Background(), "testKey", val)

	type args[D Decoder, E Encoder, T any] struct {
		ctx        context.Context
		c          ByteCache
		d          D
		e          E
		sf         *singleflight.Group
		key        string
		expiration time.Duration
		builder    func() (obj *T, err error)
	}
	type testCase[D Decoder, E Encoder, T any] struct {
		name    string
		args    args[D, E, T]
		want    *T
		wantErr bool
	}
	tests := []testCase[DecoderFunc, EncoderFunc, Example]{
		{
			name: "GetObjectEx existing key",
			args: args[DecoderFunc, EncoderFunc, Example]{
				ctx:        context.Background(),
				c:          cache,
				d:          json.Unmarshal,
				e:          json.Marshal,
				sf:         &singleflight.Group{},
				key:        "testKey",
				expiration: time.Minute,
				builder:    nil,
			},
			want:    &Example{Value: "testValue"},
			wantErr: false,
		},
		{
			name: "GetObjectEx non-existing key with builder",
			args: args[DecoderFunc, EncoderFunc, Example]{
				ctx:        context.Background(),
				c:          cache,
				d:          json.Unmarshal,
				e:          json.Marshal,
				sf:         &singleflight.Group{},
				key:        "testKeyNotFound",
				expiration: time.Minute,
				builder: func() (*Example, error) {
					return &Example{Value: "newValue"}, nil
				},
			},
			want:    &Example{Value: "newValue"},
			wantErr: false,
		},
		{
			name: "GetObjectEx with error in builder",
			args: args[DecoderFunc, EncoderFunc, Example]{
				ctx:        context.Background(),
				c:          cache,
				d:          json.Unmarshal,
				e:          json.Marshal,
				sf:         &singleflight.Group{},
				key:        "testKeyError",
				expiration: time.Minute,
				builder: func() (*Example, error) {
					return nil, errors.New("test error")
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "GetObjectEx with nil builder",
			args: args[DecoderFunc, EncoderFunc, Example]{
				ctx:        context.Background(),
				c:          cache,
				d:          json.Unmarshal,
				e:          json.Marshal,
				sf:         &singleflight.Group{},
				key:        "testKeyNilBuilder",
				expiration: time.Minute,
				builder: func() (*Example, error) {
					return nil, nil // Simulating a nil builder
				},
			},
			want:    nil,
			wantErr: false, // Expect no error when builder returns nil
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetObjectEx(tt.args.ctx, tt.args.c, tt.args.d, tt.args.e, tt.args.sf, tt.args.key, tt.args.expiration, tt.args.builder)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetObjectEx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetObjectEx() got = %v, want %v", got, tt.want)
			}
		})
	}
}
