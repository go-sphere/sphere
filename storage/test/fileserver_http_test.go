package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/fileserver"
)

func TestFileServerUploadAndDownloadOverHTTP(t *testing.T) {
	router := newMiniRouter()
	server := httptest.NewServer(router)
	defer server.Close()

	tokenCache := memory.NewByteCache()
	t.Cleanup(func() { _ = tokenCache.Close() })
	memStorage := newInMemoryStorage(t)

	fileServer, err := fileserver.NewCDNAdapter(
		fileserver.Config{
			PutBase:      server.URL + "/upload",
			GetBase:      server.URL + "/files",
			UploadNaming: storage.UploadNamingStrategyOriginal,
		},
		tokenCache,
		memStorage,
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}
	fileServer.RegisterFileUploader(router.Group("/upload"))
	fileServer.RegisterFileDownloader(router.Group("/files"))

	tokenData, err := fileServer.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		FileName: "avatar.txt",
		Dir:      "users",
	})
	if err != nil {
		t.Fatalf("GenerateUploadAuth() error = %v", err)
	}
	uploadURL := tokenData.Authorization.Value
	key := tokenData.File.Key
	downloadURL := tokenData.File.URL
	if key != "users/avatar.txt" {
		t.Fatalf("key = %q, want %q", key, "users/avatar.txt")
	}

	payload := []byte("hello over http")
	putReq, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new PUT request: %v", err)
	}
	putResp, err := server.Client().Do(putReq)
	if err != nil {
		t.Fatalf("PUT upload request failed: %v", err)
	}
	defer func() { _ = putResp.Body.Close() }()
	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		t.Fatalf("upload status = %d, want %d, body = %s", putResp.StatusCode, http.StatusOK, string(body))
	}
	var putResult httpz.DataResponse[fileserver.UploadResult]
	if err = json.NewDecoder(putResp.Body).Decode(&putResult); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if putResult.Data.Key != key {
		t.Fatalf("upload response key = %q, want %q", putResult.Data.Key, key)
	}

	getResp, err := server.Client().Get(downloadURL)
	if err != nil {
		t.Fatalf("GET download request failed: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("download status = %d, want %d, body = %s", getResp.StatusCode, http.StatusOK, string(body))
	}
	all, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	if string(all) != string(payload) {
		t.Fatalf("download body = %q, want %q", string(all), string(payload))
	}
}

type miniRoute struct {
	method  string
	pattern string
	handler httpx.Handler
}

type miniRouter struct {
	prefix string
	routes *[]miniRoute
}

func (r *miniRouter) SupportsRouterFeature(feature httpx.RouterFeature) bool {
	return true
}

func newMiniRouter() *miniRouter {
	routes := make([]miniRoute, 0, 8)
	return &miniRouter{
		prefix: "",
		routes: &routes,
	}
}

func (r *miniRouter) BasePath() string {
	return r.prefix
}

func (r *miniRouter) Group(prefix string, m ...httpx.Middleware) httpx.Router {
	return &miniRouter{
		prefix: path.Join("/", r.prefix, prefix),
		routes: r.routes,
	}
}

func (r *miniRouter) Use(...httpx.Middleware) {}

func (r *miniRouter) Handle(method, pattern string, h httpx.Handler) {
	full := path.Join("/", r.prefix, pattern)
	*r.routes = append(*r.routes, miniRoute{
		method:  method,
		pattern: full,
		handler: h,
	})
}

func (r *miniRouter) Any(pattern string, h httpx.Handler) {
	r.Handle(http.MethodGet, pattern, h)
	r.Handle(http.MethodPost, pattern, h)
	r.Handle(http.MethodPut, pattern, h)
	r.Handle(http.MethodDelete, pattern, h)
	r.Handle(http.MethodPatch, pattern, h)
	r.Handle(http.MethodHead, pattern, h)
	r.Handle(http.MethodOptions, pattern, h)
}

func (r *miniRouter) Static(prefix, root string) {}

func (r *miniRouter) StaticFS(prefix string, f fs.FS) {}

func (r *miniRouter) GET(path string, h httpx.Handler)     { r.Handle(http.MethodGet, path, h) }
func (r *miniRouter) POST(path string, h httpx.Handler)    { r.Handle(http.MethodPost, path, h) }
func (r *miniRouter) PUT(path string, h httpx.Handler)     { r.Handle(http.MethodPut, path, h) }
func (r *miniRouter) DELETE(path string, h httpx.Handler)  { r.Handle(http.MethodDelete, path, h) }
func (r *miniRouter) PATCH(path string, h httpx.Handler)   { r.Handle(http.MethodPatch, path, h) }
func (r *miniRouter) HEAD(path string, h httpx.Handler)    { r.Handle(http.MethodHead, path, h) }
func (r *miniRouter) OPTIONS(path string, h httpx.Handler) { r.Handle(http.MethodOptions, path, h) }

func (r *miniRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, rt := range *r.routes {
		if rt.method != req.Method {
			continue
		}
		params, ok := matchRoute(rt.pattern, req.URL.Path)
		if !ok {
			continue
		}
		ctx := newMiniContext(req.Context(), w, req, params)
		if err := rt.handler(ctx); err != nil {
			_, status, msg := httpx.ParseError(err)
			http.Error(w, msg, int(status))
		}
		return
	}
	http.NotFound(w, req)
}

func matchRoute(pattern string, inputPath string) (map[string]string, bool) {
	pp := splitRoute(pattern)
	sp := splitRoute(inputPath)
	params := map[string]string{}

	for i := range pp {
		if i >= len(sp) {
			if after, ok := strings.CutPrefix(pp[i], "*"); ok {
				params[after] = ""
				return params, true
			}
			return nil, false
		}
		pseg := pp[i]
		if after, ok := strings.CutPrefix(pseg, ":"); ok {
			params[after] = sp[i]
			continue
		}
		if after, ok := strings.CutPrefix(pseg, "*"); ok {
			name := after
			params[name] = "/" + strings.Join(sp[i:], "/")
			return params, true
		}
		if pseg != sp[i] {
			return nil, false
		}
	}
	if len(sp) != len(pp) {
		return nil, false
	}
	return params, true
}

func splitRoute(raw string) []string {
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		return []string{}
	}
	return parts
}

type miniContext struct {
	context.Context
	w      http.ResponseWriter
	r      *http.Request
	params map[string]string
	store  map[string]any
}

func newMiniContext(ctx context.Context, w http.ResponseWriter, r *http.Request, params map[string]string) *miniContext {
	return &miniContext{
		Context: ctx,
		w:       w,
		r:       r,
		params:  params,
		store:   map[string]any{},
	}
}

func (c *miniContext) Method() string   { return c.r.Method }
func (c *miniContext) Path() string     { return c.r.URL.Path }
func (c *miniContext) FullPath() string { return c.r.URL.Path }
func (c *miniContext) ClientIP() string { return c.r.RemoteAddr }

func (c *miniContext) Param(key string) string {
	return c.params[key]
}

func (c *miniContext) Params() map[string]string {
	out := make(map[string]string, len(c.params))
	maps.Copy(out, c.params)
	return out
}

func (c *miniContext) Query(key string) string {
	return c.r.URL.Query().Get(key)
}

func (c *miniContext) Queries() map[string][]string {
	out := map[string][]string{}
	for k, v := range c.r.URL.Query() {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func (c *miniContext) RawQuery() string {
	return c.r.URL.RawQuery
}

func (c *miniContext) Header(key string) string {
	return c.r.Header.Get(key)
}

func (c *miniContext) Headers() map[string][]string {
	out := map[string][]string{}
	for k, v := range c.r.Header {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func (c *miniContext) Cookie(name string) (string, error) {
	cookie, err := c.r.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func (c *miniContext) Cookies() map[string]string {
	out := map[string]string{}
	for _, ck := range c.r.Cookies() {
		out[ck.Name] = ck.Value
	}
	return out
}

func (c *miniContext) BodyRaw() ([]byte, error) {
	return io.ReadAll(c.r.Body)
}

func (c *miniContext) BodyReader() io.ReadCloser {
	return c.r.Body
}

func (c *miniContext) FormValue(key string) string {
	return c.r.FormValue(key)
}

func (c *miniContext) MultipartForm() (*multipart.Form, error) {
	err := c.r.ParseMultipartForm(32 << 20)
	if err != nil {
		return nil, err
	}
	return c.r.MultipartForm, nil
}

func (c *miniContext) FormFile(name string) (*multipart.FileHeader, error) {
	_, fh, err := c.r.FormFile(name)
	return fh, err
}

func (c *miniContext) BindJSON(dst any) error {
	return json.NewDecoder(c.r.Body).Decode(dst)
}

func (c *miniContext) BindQuery(dst any) error {
	return errors.New("BindQuery not implemented in miniContext")
}

func (c *miniContext) BindForm(dst any) error {
	return errors.New("BindForm not implemented in miniContext")
}

func (c *miniContext) BindURI(dst any) error {
	return errors.New("BindURI not implemented in miniContext")
}

func (c *miniContext) BindHeader(dst any) error {
	return errors.New("BindHeader not implemented in miniContext")
}

func (c *miniContext) Status(code int) {
	c.w.WriteHeader(code)
}

func (c *miniContext) SetHeader(key, value string) {
	c.w.Header().Set(key, value)
}

func (c *miniContext) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.w, cookie)
}

func (c *miniContext) JSON(code int, v any) error {
	c.w.Header().Set("Content-Type", "application/json")
	c.w.WriteHeader(code)
	return json.NewEncoder(c.w).Encode(v)
}

func (c *miniContext) Text(code int, s string) error {
	c.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.w.WriteHeader(code)
	_, err := io.WriteString(c.w, s)
	return err
}

func (c *miniContext) NoContent(code int) error {
	c.w.WriteHeader(code)
	return nil
}

func (c *miniContext) Bytes(code int, b []byte, contentType string) error {
	c.w.Header().Set("Content-Type", contentType)
	c.w.WriteHeader(code)
	_, err := c.w.Write(b)
	return err
}

func (c *miniContext) DataFromReader(code int, contentType string, r io.Reader, size int) error {
	c.w.Header().Set("Content-Type", contentType)
	c.w.WriteHeader(code)
	if size >= 0 {
		_, err := io.CopyN(c.w, r, int64(size))
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		return nil
	}
	_, err := io.Copy(c.w, r)
	return err
}

func (c *miniContext) File(filePath string) error {
	http.ServeFile(c.w, c.r, filePath)
	return nil
}

func (c *miniContext) Redirect(code int, location string) error {
	http.Redirect(c.w, c.r, location, code)
	return nil
}

func (c *miniContext) Set(key string, val any) {
	c.store[key] = val
}

func (c *miniContext) Get(key string) (any, bool) {
	val, ok := c.store[key]
	return val, ok
}

func (c *miniContext) Next() error {
	return nil
}
