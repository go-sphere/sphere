// Package fakeqiniu is an in-process emulation of the Qiniu Kodo endpoints the
// qiniu storage driver reaches through the official SDK: the uc region query,
// form upload (up), object management (rs: stat/delete/copy/move), listing
// (rsf) and source download (io). It exists so the driver can be exercised end
// to end in tests without network access or credentials.
//
// The SDK discovers every service host from the uc query, and the uc host list
// is process-global (qiniuStorage.SetUcHosts). A single shared uc server is
// therefore started on first use and routes each query to the fake registered
// for the access key in it; every fake gets a unique random access key, so
// fakes can be used from parallel tests and never collide in the SDK's
// in-memory region cache. The same first use also turns off the SDK's other
// process-global side effects: usage logging (buffered under $TMPDIR and
// uploaded to Qiniu) and the region cache it persists under $TMPDIR.
//
// Emulated semantics (per the Kodo API documentation): rs errors use Qiniu's
// own status codes (612 missing key, 614 destination exists); downloads of a
// missing key answer HTTP 404; upload tokens are verified (signature,
// deadline, scope, insertOnly), and a bucket-only scope is insert-only, so an
// upload that would replace different content fails with 614.
package fakeqiniu

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	qiniuStorage "github.com/qiniu/go-sdk/v7/storage"
	"github.com/qiniu/go-sdk/v7/storagev2/uplog"
)

// Object is a stored object as seen by the fake.
type Object struct {
	Data    []byte
	MIME    string
	PutTime int64
}

// Server is one fake Kodo bucket.
type Server struct {
	AccessKey string
	SecretKey string
	Bucket    string

	api *httptest.Server // up + rs + rsf
	io  *httptest.Server // source download

	mu      sync.Mutex
	objects map[string]Object
	ioHook  func(*http.Request)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]*Server{}

	// ucServer is shared by every fake in the process because the SDK reads the
	// uc host list from a package-level variable.
	ucServer = sync.OnceValue(func() *httptest.Server {
		uplog.DisableUplog()
		// The SDK silently skips persistence when it cannot create the
		// cache directory, which a path under the null device guarantees.
		qiniuStorage.SetRegionCachePath(filepath.Join(os.DevNull, "region.cache.json"))
		srv := httptest.NewServer(withReqID(http.HandlerFunc(serveUC)))
		qiniuStorage.SetUcHosts(srv.URL)
		return srv
	})
)

// New starts a fake bucket and registers it for the lifetime of tb.
func New(tb testing.TB, bucket string) *Server {
	tb.Helper()
	ucServer()
	s := &Server{
		AccessKey: "fake-ak-" + rand.Text(),
		SecretKey: "fake-sk-" + rand.Text(),
		Bucket:    bucket,
		objects:   map[string]Object{},
	}
	s.api = httptest.NewServer(withReqID(http.HandlerFunc(s.serveAPI)))
	s.io = httptest.NewServer(withReqID(http.HandlerFunc(s.serveIO)))
	registryMu.Lock()
	registry[s.AccessKey] = s
	registryMu.Unlock()
	tb.Cleanup(func() {
		registryMu.Lock()
		delete(registry, s.AccessKey)
		registryMu.Unlock()
		s.api.Close()
		s.io.Close()
	})
	return s
}

// Object returns a copy of the stored object for key.
func (s *Server) Object(key string) (Object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, ok := s.objects[key]
	obj.Data = bytes.Clone(obj.Data)
	return obj, ok
}

// Put seeds an object directly, bypassing the upload endpoint.
func (s *Server) Put(key string, data []byte, mimeType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = Object{Data: bytes.Clone(data), MIME: mimeType, PutTime: time.Now().UnixNano() / 100}
}

// SetIOHook installs fn to run before every download (io) request is served,
// e.g. to replace an object between the SDK's HEAD and its ranged GET.
func (s *Server) SetIOHook(fn func(*http.Request)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ioHook = fn
}

// UploadURL is the form-upload endpoint, for posting a client-side upload
// with a token issued by GenerateUploadAuth.
func (s *Server) UploadURL() string { return s.api.URL }

// Policy is the subset of a Qiniu PutPolicy the fake enforces.
type Policy struct {
	Scope      string `json:"scope"`
	Deadline   int64  `json:"deadline"`
	InsertOnly int    `json:"insertOnly"`
	MimeLimit  string `json:"mimeLimit"`
}

// VerifyUploadToken checks the token's signature against the fake's secret key
// and decodes its policy. It does not check the deadline.
func (s *Server) VerifyUploadToken(token string) (Policy, error) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return Policy{}, errors.New("malformed upload token")
	}
	if parts[0] != s.AccessKey {
		return Policy{}, fmt.Errorf("unknown access key %q", parts[0])
	}
	mac := hmac.New(sha1.New, []byte(s.SecretKey))
	mac.Write([]byte(parts[2]))
	if want := base64.URLEncoding.EncodeToString(mac.Sum(nil)); !hmac.Equal([]byte(want), []byte(parts[1])) {
		return Policy{}, errors.New("bad upload token signature")
	}
	raw, err := base64.URLEncoding.DecodeString(parts[2])
	if err != nil {
		return Policy{}, fmt.Errorf("decode policy: %w", err)
	}
	var policy Policy
	if err := json.Unmarshal(raw, &policy); err != nil {
		return Policy{}, fmt.Errorf("parse policy: %w", err)
	}
	return policy, nil
}

func serveUC(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	registryMu.RLock()
	s := registry[q.Get("ak")]
	registryMu.RUnlock()
	if s == nil || q.Get("bucket") != s.Bucket {
		writeError(w, http.StatusNotFound, "no such bucket")
		return
	}
	api := strings.TrimPrefix(s.api.URL, "http://")
	io := strings.TrimPrefix(s.io.URL, "http://")
	switch r.URL.Path {
	case "/v2/query":
		writeJSON(w, http.StatusOK, map[string]any{
			"region": "fake", "ttl": 86400,
			"up":     map[string]any{"src": map[string]any{"main": []string{api}}},
			"io":     map[string]any{"src": map[string]any{"main": []string{io}}},
			"io_src": map[string]any{"src": map[string]any{"main": []string{io}}},
			"rs":     map[string]any{"acc": map[string]any{"main": []string{api}}},
			"rsf":    map[string]any{"acc": map[string]any{"main": []string{api}}},
			"api":    map[string]any{"acc": map[string]any{"main": []string{api}}},
		})
	case "/v4/query":
		domains := func(h string) map[string]any { return map[string]any{"domains": []string{h}} }
		writeJSON(w, http.StatusOK, map[string]any{"hosts": []any{map[string]any{
			"region": "fake", "ttl": 86400,
			"up": domains(api), "io": domains(io), "io_src": domains(io),
			"rs": domains(api), "rsf": domains(api), "api": domains(api),
			"uc": domains(r.Host),
		}}})
	default:
		writeError(w, http.StatusNotFound, "unknown uc path "+r.URL.Path)
	}
}

func (s *Server) serveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/" {
		s.serveUpload(w, r)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu "+s.AccessKey+":") {
		writeError(w, http.StatusUnauthorized, "bad token")
		return
	}
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.URL.Path == "/list":
		s.serveList(w, r)
	case segments[0] == "stat" && len(segments) == 2:
		key, ok := s.decodeEntry(w, segments[1])
		if !ok {
			return
		}
		s.mu.Lock()
		obj, exists := s.objects[key]
		s.mu.Unlock()
		if !exists {
			writeError(w, 612, "no such file or directory")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"fsize": len(obj.Data), "hash": etag(obj.Data), "mimeType": obj.MIME, "putTime": obj.PutTime, "type": 0,
		})
	case segments[0] == "delete" && len(segments) == 2:
		key, ok := s.decodeEntry(w, segments[1])
		if !ok {
			return
		}
		s.mu.Lock()
		_, exists := s.objects[key]
		delete(s.objects, key)
		s.mu.Unlock()
		if !exists {
			writeError(w, 612, "no such file or directory")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{})
	case (segments[0] == "copy" || segments[0] == "move") && (len(segments) == 3 || len(segments) == 5):
		src, ok := s.decodeEntry(w, segments[1])
		if !ok {
			return
		}
		dst, ok := s.decodeEntry(w, segments[2])
		if !ok {
			return
		}
		force := len(segments) == 5 && segments[3] == "force" && segments[4] == "true"
		s.mu.Lock()
		defer s.mu.Unlock()
		obj, exists := s.objects[src]
		if !exists {
			writeError(w, 612, "no such file or directory")
			return
		}
		if _, taken := s.objects[dst]; taken && !force {
			writeError(w, 614, "file exists")
			return
		}
		s.objects[dst] = obj
		if segments[0] == "move" && src != dst {
			delete(s.objects, src)
		}
		writeJSON(w, http.StatusOK, map[string]any{})
	default:
		writeError(w, http.StatusNotFound, "unknown rs path "+r.URL.Path)
	}
}

func (s *Server) decodeEntry(w http.ResponseWriter, encoded string) (string, bool) {
	raw, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid argument")
		return "", false
	}
	bucket, key, found := strings.Cut(string(raw), ":")
	if !found || bucket != s.Bucket {
		writeError(w, 631, "no such bucket")
		return "", false
	}
	return key, true
}

func (s *Server) serveList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("bucket") != s.Bucket {
		writeError(w, 631, "no such bucket")
		return
	}
	prefix := q.Get("prefix")
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 1000
	}
	after := ""
	if marker := q.Get("marker"); marker != "" {
		raw, err := base64.URLEncoding.DecodeString(marker)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid marker")
			return
		}
		after = string(raw)
	}
	s.mu.Lock()
	keys := make([]string, 0, len(s.objects))
	for key := range s.objects {
		if strings.HasPrefix(key, prefix) && key > after {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	items := make([]map[string]any, 0, min(limit, len(keys)))
	for _, key := range keys[:min(limit, len(keys))] {
		obj := s.objects[key]
		items = append(items, map[string]any{
			"key": key, "fsize": len(obj.Data), "hash": etag(obj.Data), "mimeType": obj.MIME, "putTime": obj.PutTime,
		})
	}
	s.mu.Unlock()
	// The marker is opaque on purpose (not the bare last key) so callers that
	// construct cursors themselves are caught.
	marker := ""
	if len(keys) > limit {
		marker = base64.URLEncoding.EncodeToString([]byte(keys[limit-1]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"marker": marker, "items": items, "commonPrefixes": []string{}})
}

func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "bad multipart: "+err.Error())
		return
	}
	policy, err := s.VerifyUploadToken(r.FormValue("token"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if time.Now().Unix() > policy.Deadline {
		writeError(w, http.StatusUnauthorized, "expired token")
		return
	}
	bucket, scopedKey, keyScoped := strings.Cut(policy.Scope, ":")
	if bucket != s.Bucket {
		writeError(w, http.StatusUnauthorized, "scope bucket mismatch")
		return
	}
	key := r.FormValue("key")
	if keyScoped && key != scopedKey {
		writeError(w, http.StatusForbidden, "key doesn't match with scope")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read file")
		return
	}
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = mime.TypeByExtension(path.Ext(key))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if policy.MimeLimit != "" && !mimeAllowed(policy.MimeLimit, mimeType) {
		writeError(w, 403, "limited mimeType: this file type ("+mimeType+") is forbidden to upload")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, exists := s.objects[key]; exists {
		// "bucket" scope is insert-only; "bucket:key" may overwrite unless
		// insertOnly is set. Re-uploading identical content is not a conflict.
		if (!keyScoped || policy.InsertOnly != 0) && !bytes.Equal(existing.Data, data) {
			writeError(w, 614, "file exists")
			return
		}
	}
	s.objects[key] = Object{Data: data, MIME: mimeType, PutTime: time.Now().UnixNano() / 100}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "hash": etag(data)})
}

func (s *Server) serveIO(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	hook := s.ioHook
	s.mu.Unlock()
	if hook != nil {
		hook(r)
	}
	key, err := url.PathUnescape(strings.TrimPrefix(r.URL.EscapedPath(), "/"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad path")
		return
	}
	s.mu.Lock()
	obj, exists := s.objects[key]
	s.mu.Unlock()
	if !exists {
		writeError(w, http.StatusNotFound, "Document not found")
		return
	}
	w.Header().Set("Content-Type", obj.MIME)
	w.Header().Set("ETag", `"`+etag(obj.Data)+`"`)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", time.Unix(0, obj.PutTime*100).UTC().Format(http.TimeFormat))
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(obj.Data))
}

func mimeAllowed(limit, mimeType string) bool {
	base, _, _ := strings.Cut(mimeType, ";")
	for pattern := range strings.SplitSeq(limit, ";") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*/*" || pattern == base {
			return true
		}
		if prefix, ok := strings.CutSuffix(pattern, "/*"); ok && strings.HasPrefix(base, prefix+"/") {
			return true
		}
	}
	return false
}

func withReqID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The SDK's anti-hijacking interceptor rejects responses without one.
		w.Header().Set("X-Reqid", rand.Text())
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func etag(data []byte) string {
	sum := sha1.Sum(data)
	return base64.URLEncoding.EncodeToString(sum[:])
}
