package requestid

import (
	"strings"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/log"
)

type seen struct {
	store, ctx string
	attrs      []log.Attr
}

func run(t *testing.T, header string, opts ...Option) (*httpxmock.Context, seen) {
	t.Helper()
	var opt []httpxmock.Option
	opt = append(opt, httpxmock.WithContext(t.Context()))
	if header != "" {
		opt = append(opt, httpxmock.WithHeader(DefaultHeader, header))
	}
	c := httpxmock.NewRequest("GET", "/x", nil, opt...)
	var s seen
	next := func(ctx httpx.Context) error {
		v, _ := ctx.Get(StoreKey)
		s.store, _ = v.(string)
		s.ctx = FromContext(ctx.Context())
		s.attrs = log.AttrsFromContext(ctx.Context())
		return nil
	}
	if err := httpxmock.Run(c, next, New(opts...)); err != nil {
		t.Fatal(err)
	}
	return c, s
}

func TestAcceptsValidClientID(t *testing.T) {
	t.Parallel()
	c, s := run(t, "abc.DEF_123-x")
	if s.store != "abc.DEF_123-x" || s.ctx != s.store {
		t.Fatalf("store=%q ctx=%q", s.store, s.ctx)
	}
	if got := c.ResponseHeader(DefaultHeader); got != s.store {
		t.Fatalf("response header = %q", got)
	}
	if len(s.attrs) != 1 || s.attrs[0].Key != LogKey || s.attrs[0].Value.String() != s.store {
		t.Fatalf("attrs = %v", s.attrs)
	}
}

func TestRejectsInvalidClientID(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"has space", "new\nline", "emoji-é", strings.Repeat("a", MaxLength+1)} {
		c, s := run(t, in)
		if s.store == "" || s.store == in {
			t.Errorf("input %q: id = %q, want regenerated", in, s.store)
		}
		if c.ResponseHeader(DefaultHeader) != s.store || s.ctx != s.store {
			t.Errorf("input %q: channels disagree", in)
		}
	}
	_, s := run(t, strings.Repeat("a", MaxLength))
	if len(s.store) != MaxLength {
		t.Errorf("64-char ID should be accepted, got %q", s.store)
	}
}

func TestGeneratesWhenAbsent(t *testing.T) {
	t.Parallel()
	_, a := run(t, "")
	_, b := run(t, "")
	if len(a.store) != 32 || a.store == b.store {
		t.Fatalf("a=%q b=%q", a.store, b.store)
	}
}

func TestAlwaysGenerateIgnoresClient(t *testing.T) {
	t.Parallel()
	_, s := run(t, "client-id", WithAlwaysGenerate())
	if s.store == "client-id" || s.store == "" {
		t.Fatalf("id = %q", s.store)
	}
}

func TestCustomHeaderAndGenerator(t *testing.T) {
	t.Parallel()
	c := httpxmock.NewRequest("GET", "/x", nil, httpxmock.WithContext(t.Context()))
	next := func(httpx.Context) error { return nil }
	err := httpxmock.Run(c, next, New(WithHeader("X-Trace"), WithGenerator(func() string { return "fixed" })))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ResponseHeader("X-Trace"); got != "fixed" {
		t.Fatalf("header = %q", got)
	}
}
