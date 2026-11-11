// Package fakes3 is a minimal in-process S3 endpoint for exercising the s3
// storage driver (minio-go) in tests without network access or credentials.
//
// It serves one path-style bucket and implements just what the driver calls:
// GetBucketLocation, PutObject (single and multipart), HeadObject, GetObject,
// DeleteObject, CopyObject and ListObjectsV2. Missing keys answer NoSuchKey
// like real S3; DeleteObject of a missing key is a 204 no-op; copying an
// object onto itself without a REPLACE metadata directive answers
// InvalidRequest. UploadPartCopy serves multipart copies. Requests signed in
// the Authorization header are not authenticated; presigned URLs are verified
// when Server.SecretKey is set.
package fakes3

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
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
	// SecretKey, when set, makes the fake verify the SigV4 query signature of
	// presigned requests (those carrying X-Amz-Signature), including every
	// signed header, and answer SignatureDoesNotMatch on a mismatch.
	SecretKey string

	srv *httptest.Server

	mu         sync.Mutex
	objects    map[string]Object
	uploads    map[string]map[int][]byte
	uploadMIME map[string]string
	nextUpload int
	largest    int      // largest multipart part body received
	maxKeys    []int    // max-keys of each ListObjectsV2 request, 0 when absent
	ops        []string // S3 operation name of each object request, in order
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

// LargestPart returns the size of the largest multipart part received, which
// is the client's per-upload part buffer.
func (s *Server) LargestPart() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.largest
}

// ListMaxKeys returns the max-keys parameter of every ListObjectsV2 request
// received so far, in order; 0 means the request did not set one.
func (s *Server) ListMaxKeys() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.maxKeys)
}

// Ops returns the S3 operation name of every object-level request received
// so far, in order: PutObject, CreateMultipartUpload, UploadPart,
// UploadPartCopy, CompleteMultipartUpload, AbortMultipartUpload, CopyObject,
// HeadObject, GetObject or DeleteObject.
func (s *Server) Ops() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.ops)
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
	if s.SecretKey != "" && q.Has("X-Amz-Signature") && !s.validPresignature(r) {
		writeError(w, http.StatusForbidden, "SignatureDoesNotMatch", "The request signature we calculated does not match the signature you provided.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ops = append(s.ops, operation(r.Method, q, r.Header))
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
		if id := q.Get("uploadId"); id != "" && r.Header.Get("X-Amz-Copy-Source") != "" {
			s.uploadPartCopy(w, r, id)
			return
		}
		if id := q.Get("uploadId"); id != "" {
			n, _ := strconv.Atoi(q.Get("partNumber"))
			body := readBody(r)
			if s.uploads[id] == nil {
				s.uploads[id] = map[int][]byte{}
			}
			s.uploads[id][n] = body
			s.largest = max(s.largest, len(body))
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
			if srcKey == key && r.Header.Get("X-Amz-Metadata-Directive") != "REPLACE" {
				writeError(w, http.StatusBadRequest, "InvalidRequest", "This copy request is illegal because it is trying to copy an object to itself without changing the object's metadata, storage class, website redirect location or encryption attributes.")
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
		// Aborting a multipart upload (minio-go does so when the source
		// reader fails) discards its parts and leaves the stored object alone.
		if id := q.Get("uploadId"); id != "" {
			delete(s.uploads, id)
			delete(s.uploadMIME, id)
		} else {
			delete(s.objects, key)
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// operation names the S3 API an object-level request calls.
func operation(method string, q url.Values, h http.Header) string {
	copySource := h.Get("X-Amz-Copy-Source") != ""
	switch {
	case method == http.MethodPost && q.Has("uploads"):
		return "CreateMultipartUpload"
	case method == http.MethodPost && q.Has("uploadId"):
		return "CompleteMultipartUpload"
	case method == http.MethodPut && q.Has("uploadId") && copySource:
		return "UploadPartCopy"
	case method == http.MethodPut && q.Has("uploadId"):
		return "UploadPart"
	case method == http.MethodPut && copySource:
		return "CopyObject"
	case method == http.MethodPut:
		return "PutObject"
	case method == http.MethodDelete && q.Has("uploadId"):
		return "AbortMultipartUpload"
	case method == http.MethodDelete:
		return "DeleteObject"
	case method == http.MethodHead:
		return "HeadObject"
	case method == http.MethodGet:
		return "GetObject"
	default:
		return method
	}
}

// uploadPartCopy stores a byte range of an existing object as a part of the
// multipart upload id. The caller holds s.mu.
func (s *Server) uploadPartCopy(w http.ResponseWriter, r *http.Request, id string) {
	source, _ := url.PathUnescape(r.Header.Get("X-Amz-Copy-Source"))
	srcBucket, srcKey, _ := strings.Cut(strings.TrimPrefix(source, "/"), "/")
	obj, ok := s.objects[srcKey]
	if srcBucket != s.Bucket || !ok {
		writeError(w, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
		return
	}
	if match := r.Header.Get("X-Amz-Copy-Source-If-Match"); match != "" && strings.Trim(match, `"`) != strings.Trim(etag(obj.Data), `"`) {
		writeError(w, http.StatusPreconditionFailed, "PreconditionFailed", "At least one of the pre-conditions you specified did not hold.")
		return
	}
	data := obj.Data
	if rng := r.Header.Get("X-Amz-Copy-Source-Range"); rng != "" {
		var start, end int
		if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || end >= len(data) {
			writeError(w, http.StatusBadRequest, "InvalidArgument", "Invalid copy source range.")
			return
		}
		data = data[start : end+1]
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("partNumber"))
	if s.uploads[id] == nil {
		s.uploads[id] = map[int][]byte{}
	}
	s.uploads[id][n] = bytes.Clone(data)
	writeXML(w, http.StatusOK, struct {
		XMLName      xml.Name `xml:"CopyPartResult"`
		ETag         string
		LastModified string
	}{ETag: etag(data), LastModified: time.Now().UTC().Format(time.RFC3339)})
}

// validPresignature checks a SigV4 query-string signature (as produced by
// minio-go's Presign family) against s.SecretKey. Presigned payloads are
// always UNSIGNED-PAYLOAD.
func (s *Server) validPresignature(r *http.Request) bool {
	q := r.URL.Query()
	signature := q.Get("X-Amz-Signature")
	q.Del("X-Amz-Signature")
	amzDate := q.Get("X-Amz-Date")
	signed, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return false
	}
	expires, err := strconv.Atoi(q.Get("X-Amz-Expires"))
	if err != nil || time.Now().After(signed.Add(time.Duration(expires)*time.Second)) {
		return false
	}
	_, scope, ok := strings.Cut(q.Get("X-Amz-Credential"), "/")
	if !ok {
		return false
	}
	scopeParts := strings.Split(scope, "/")
	if len(scopeParts) != 4 {
		return false
	}

	signedHeaders := q.Get("X-Amz-SignedHeaders")
	var headers strings.Builder
	for name := range strings.SplitSeq(signedHeaders, ";") {
		value := r.Header.Get(name)
		if name == "host" {
			value = r.Host
		}
		headers.WriteString(name + ":" + strings.TrimSpace(value) + "\n")
	}
	names := slices.Sorted(maps.Keys(q))
	query := make([]string, 0, len(names))
	for _, name := range names {
		for _, value := range q[name] {
			query = append(query, awsEscape(name, false)+"="+awsEscape(value, false))
		}
	}
	canonical := strings.Join([]string{
		r.Method,
		awsEscape(r.URL.Path, true),
		strings.Join(query, "&"),
		headers.String(),
		signedHeaders,
		"UNSIGNED-PAYLOAD",
	}, "\n")
	digest := sha256.Sum256([]byte(canonical))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(digest[:])

	key := []byte("AWS4" + s.SecretKey)
	for _, part := range scopeParts {
		key = hmacSHA256(key, part)
	}
	want := hex.EncodeToString(hmacSHA256(key, stringToSign))
	return hmac.Equal([]byte(want), []byte(signature))
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// awsEscape percent-encodes s the way SigV4 canonicalization does: everything
// but unreserved characters, and "/" too unless keepSlash is set.
func awsEscape(s string, keepSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9',
			c == '-', c == '_', c == '.', c == '~', keepSlash && c == '/':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
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
	requested, _ := strconv.Atoi(q.Get("max-keys"))
	if requested > 0 {
		limit = min(limit, requested)
	}

	s.mu.Lock()
	s.maxKeys = append(s.maxKeys, requested)
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
