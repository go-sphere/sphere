// Package fakes3 is a minimal in-process S3 endpoint for exercising the s3
// storage driver (minio-go) in tests without network access or credentials.
//
// It serves one path-style bucket and implements just what the driver calls:
// GetBucketLocation, PutObject (single and multipart), HeadObject, GetObject,
// DeleteObject, CopyObject and ListObjectsV2. Missing keys answer NoSuchKey
// like real S3; DeleteObject of a missing key is a 204 no-op. Requests are not
// authenticated.
package fakes3

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Object is a stored object as seen by the fake.
type Object struct {
	Data []byte
	MIME string
}

// Server is a fake single-bucket S3 endpoint.
type Server struct {
	Bucket string
	// MaxKeys caps the page size of ListObjectsV2 regardless of the requested
	// max-keys, so tests can force the client through continuation tokens.
	MaxKeys int

	srv *httptest.Server

	mu         sync.Mutex
	objects    map[string]Object
	uploads    map[string]map[int][]byte
	uploadMIME map[string]string
	nextUpload int
}

// New starts a fake bucket for the lifetime of tb.
func New(tb testing.TB, bucket string) *Server {
	tb.Helper()
	s := &Server{
		Bucket:     bucket,
		MaxKeys:    1000,
		objects:    map[string]Object{},
		uploads:    map[string]map[int][]byte{},
		uploadMIME: map[string]string{},
	}
	s.srv = httptest.NewServer(s)
	tb.Cleanup(s.srv.Close)
	return s
}

// Endpoint is the host:port to configure the driver with (UseSSL false).
func (s *Server) Endpoint() string { return strings.TrimPrefix(s.srv.URL, "http://") }

// Object returns a copy of the stored object for key.
func (s *Server) Object(key string) (Object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, ok := s.objects[key]
	obj.Data = bytes.Clone(obj.Data)
	return obj, ok
}

// Put seeds an object directly.
func (s *Server) Put(key string, data []byte, mimeType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = Object{Data: bytes.Clone(data), MIME: mimeType}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if bucket != s.Bucket {
		writeError(w, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}
	q := r.URL.Query()
	if key == "" {
		switch {
		case q.Has("location"):
			writeXML(w, http.StatusOK, struct {
				XMLName xml.Name `xml:"LocationConstraint"`
				Value   string   `xml:",chardata"`
			}{Value: "us-east-1"})
		case r.Method == http.MethodGet && q.Get("list-type") == "2":
			s.listV2(w, q)
		default:
			w.WriteHeader(http.StatusOK)
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodPost:
		switch {
		case q.Has("uploads"):
			s.nextUpload++
			id := "upload-" + strconv.Itoa(s.nextUpload)
			s.uploads[id] = map[int][]byte{}
			s.uploadMIME[id] = r.Header.Get("Content-Type")
			writeXML(w, http.StatusOK, struct {
				XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
				Bucket   string
				Key      string
				UploadId string
			}{Bucket: bucket, Key: key, UploadId: id})
		case q.Get("uploadId") != "":
			id := q.Get("uploadId")
			parts := s.uploads[id]
			var buf bytes.Buffer
			for i := 1; i <= len(parts); i++ {
				buf.Write(parts[i])
			}
			s.objects[key] = Object{Data: buf.Bytes(), MIME: s.uploadMIME[id]}
			delete(s.uploads, id)
			delete(s.uploadMIME, id)
			writeXML(w, http.StatusOK, struct {
				XMLName xml.Name `xml:"CompleteMultipartUploadResult"`
				Bucket  string
				Key     string
				ETag    string
			}{Bucket: bucket, Key: key, ETag: etag(buf.Bytes())})
		default:
			w.WriteHeader(http.StatusOK)
		}

	case http.MethodPut:
		if id := q.Get("uploadId"); id != "" {
			n, _ := strconv.Atoi(q.Get("partNumber"))
			body := readBody(r)
			if s.uploads[id] == nil {
				s.uploads[id] = map[int][]byte{}
			}
			s.uploads[id][n] = body
			w.Header().Set("ETag", etag(body))
			w.WriteHeader(http.StatusOK)
			return
		}
		if source := r.Header.Get("X-Amz-Copy-Source"); source != "" {
			source, _ = url.PathUnescape(source)
			srcBucket, srcKey, _ := strings.Cut(strings.TrimPrefix(source, "/"), "/")
			obj, ok := s.objects[srcKey]
			if srcBucket != s.Bucket || !ok {
				writeError(w, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
				return
			}
			obj.Data = bytes.Clone(obj.Data)
			s.objects[key] = obj
			writeXML(w, http.StatusOK, struct {
				XMLName      xml.Name `xml:"CopyObjectResult"`
				ETag         string
				LastModified string
			}{ETag: etag(obj.Data), LastModified: time.Now().UTC().Format(time.RFC3339)})
			return
		}
		body := readBody(r)
		s.objects[key] = Object{Data: body, MIME: r.Header.Get("Content-Type")}
		w.Header().Set("ETag", etag(body))
		w.WriteHeader(http.StatusOK)

	case http.MethodHead, http.MethodGet:
		obj, ok := s.objects[key]
		if !ok {
			if r.Method == http.MethodHead {
				// HEAD has no body; minio-go maps a bare 404 to NoSuchKey.
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeError(w, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
			return
		}
		if obj.MIME != "" {
			w.Header().Set("Content-Type", obj.MIME)
		}
		w.Header().Set("ETag", etag(obj.Data))
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(obj.Data))

	case http.MethodDelete:
		delete(s.objects, key)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type listContent struct {
	Key          string
	LastModified string
	ETag         string
	Size         int
	StorageClass string
}

func (s *Server) listV2(w http.ResponseWriter, q url.Values) {
	prefix := q.Get("prefix")
	after := q.Get("start-after")
	if token := q.Get("continuation-token"); token != "" {
		after = max(after, token)
	}
	limit := s.MaxKeys
	if n, err := strconv.Atoi(q.Get("max-keys")); err == nil && n > 0 {
		limit = min(limit, n)
	}

	s.mu.Lock()
	keys := make([]string, 0, len(s.objects))
	for key := range s.objects {
		if strings.HasPrefix(key, prefix) && key > after {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	page := keys[:min(limit, len(keys))]
	contents := make([]listContent, 0, len(page))
	for _, key := range page {
		obj := s.objects[key]
		contents = append(contents, listContent{
			Key: key, LastModified: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
			ETag: etag(obj.Data), Size: len(obj.Data), StorageClass: "STANDARD",
		})
	}
	s.mu.Unlock()

	truncated := len(keys) > len(page)
	next := ""
	if truncated {
		next = page[len(page)-1]
	}
	writeXML(w, http.StatusOK, struct {
		XMLName               xml.Name `xml:"ListBucketResult"`
		Name                  string
		Prefix                string
		StartAfter            string `xml:",omitempty"`
		KeyCount              int
		MaxKeys               int
		IsTruncated           bool
		ContinuationToken     string `xml:",omitempty"`
		NextContinuationToken string `xml:",omitempty"`
		Contents              []listContent
	}{
		Name: s.Bucket, Prefix: prefix, StartAfter: q.Get("start-after"), KeyCount: len(contents),
		MaxKeys: limit, IsTruncated: truncated, ContinuationToken: q.Get("continuation-token"),
		NextContinuationToken: next, Contents: contents,
	})
}

// readBody returns the request payload, decoding the aws-chunked framing
// minio-go uses for streaming-signed uploads over plain HTTP. Chunk signatures
// are not verified.
func readBody(r *http.Request) []byte {
	body, _ := io.ReadAll(r.Body)
	if !strings.HasPrefix(r.Header.Get("X-Amz-Content-Sha256"), "STREAMING-") {
		return body
	}
	var out []byte
	for len(body) > 0 {
		header, rest, ok := bytes.Cut(body, []byte("\r\n"))
		if !ok {
			break
		}
		sizeHex, _, _ := bytes.Cut(header, []byte(";"))
		size, err := strconv.ParseInt(string(sizeHex), 16, 64)
		if err != nil || size == 0 || int64(len(rest)) < size {
			break
		}
		out = append(out, rest[:size]...)
		body = bytes.TrimPrefix(rest[size:], []byte("\r\n"))
	}
	return out
}

func writeXML(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeXML(w, status, struct {
		XMLName xml.Name `xml:"Error"`
		Code    string
		Message string
	}{Code: code, Message: msg})
}

func etag(data []byte) string {
	sum := md5.Sum(data)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}
