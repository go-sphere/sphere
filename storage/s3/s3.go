package s3

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/go-sphere/sphere/storage/urlhandler"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds the configuration parameters for S3-compatible object storage.
type Config struct {
	// Endpoint is the host[:port] of the S3 API, without scheme
	// (for example "s3.amazonaws.com" or "localhost:9000").
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	// AccessKeyID, SecretAccessKey, and the optional session Token are static
	// V4 credentials.
	AccessKeyID     string `json:"access_key" yaml:"access_key"`
	SecretAccessKey string `json:"secret" yaml:"secret"`
	Token           string `json:"token" yaml:"token"`
	// Bucket is the bucket every key lives in. It must already exist.
	Bucket string `json:"bucket" yaml:"bucket"`
	// UseSSL selects https for the API and for the derived PublicBase.
	UseSSL bool `json:"use_ssl" yaml:"use_ssl"`
	// PublicBase is the absolute http(s) base URL for GenerateURL. Empty means
	// "http(s)://Endpoint/Bucket" (path-style).
	PublicBase string `json:"public_base" yaml:"public_base"`
	// Dir is the prefix directory for keys created by GenerateUploadAuth.
	// It does not affect UploadFile or other key-based methods.
	Dir string `json:"dir" yaml:"dir"`
	// UploadNaming selects how GenerateUploadAuth names files; empty means
	// storage.UploadNamingStrategyRandomExt.
	UploadNaming storage.UploadNamingStrategy `json:"upload_naming" yaml:"upload_naming"`
	// UploadTTL is the default validity window for presigned upload URLs, and
	// also the ceiling for UploadAuthRequest.TTL. A zero value falls back to
	// defaultUploadTTL.
	UploadTTL time.Duration `json:"upload_ttl" yaml:"upload_ttl"`
	// PartSize is the multipart part size for UploadFile, whose reader has an
	// unknown length. UploadFile buffers at most one part in memory per upload
	// and sends a body no longer than PartSize with a single PutObject. The
	// largest object UploadFile accepts is PartSize * 10000. A zero value
	// falls back to defaultPartSize; the minimum is 5 MiB.
	PartSize uint64 `json:"part_size" yaml:"part_size"`
}

const (
	// defaultUploadTTL is the presigned upload URL validity used when neither
	// the request nor the config specifies one.
	defaultUploadTTL = time.Hour
	// defaultPartSize caps UploadFile's per-upload buffer at 16 MiB, allowing
	// objects up to ~156 GiB.
	defaultPartSize = 16 << 20
	// minPartSize is the smallest part S3 accepts.
	minPartSize = 5 << 20
	// maxUploadParts is the most parts an S3 multipart upload may have.
	maxUploadParts = 10000
	// initialPartBuffer is the first allocation UploadFile makes for a body.
	initialPartBuffer = 64 << 10
	// maxListKeys is the most keys S3 returns per ListObjectsV2 request.
	maxListKeys = 1000
)

// maxCopyObjectSize is the largest source CopyFile copies with one CopyObject
// request, which S3 caps at 5 GiB; larger sources are copied in parts of at
// most this size. A var so tests can lower it.
var maxCopyObjectSize int64 = 5 << 30

// Client is a minio-go storage.CDNStorage: presigned PUT upload auth and
// the core Storage operations. It also implements storage.FileStater and
// storage.FileLister. Create it with NewClient; it is safe for concurrent use
// to the extent minio-go's client is, and has no Close.
type Client struct {
	urlhandler.Handler
	config Config
	client *minio.Client
}

// NewClient creates a minio-backed storage client. An empty PublicBase is
// derived from endpoint+bucket. The minio client is constructed here; this
// type has no Close. NewClient does not contact the server, so bad
// credentials or a missing bucket surface on the first operation. It returns
// an error for a PartSize below 5 MiB, an invalid Endpoint, or a PublicBase
// that is not an absolute http or https URL.
func NewClient(conf Config) (*Client, error) {
	conf.PartSize = cmp.Or(conf.PartSize, defaultPartSize)
	if conf.PartSize < minPartSize {
		return nil, fmt.Errorf("s3: part size %d is below the 5 MiB minimum", conf.PartSize)
	}
	client, err := minio.New(conf.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(conf.AccessKeyID, conf.SecretAccessKey, conf.Token),
		Secure: conf.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	if conf.PublicBase == "" {
		if conf.UseSSL {
			conf.PublicBase = "https://" + conf.Endpoint + "/" + conf.Bucket
		} else {
			conf.PublicBase = "http://" + conf.Endpoint + "/" + conf.Bucket
		}
	}
	handler, err := urlhandler.NewHandler(conf.PublicBase)
	if err != nil {
		return nil, err
	}
	return &Client{
		Handler: *handler,
		config:  conf,
		client:  client,
	}, nil
}

// GenerateUploadAuth creates a presigned PUT URL for direct client uploads to S3.
// It generates the storage key using configured naming strategy and returns
// the presigned URL, storage key, and public access URL. The presigned URL
// validity defaults to Config.UploadTTL (else defaultUploadTTL); req.TTL may
// shorten it but never extend it.
//
// The URL signs a Content-Type, returned in Authorization.Headers; the client
// must send it unchanged or S3 rejects the upload. It is the type of the
// file name's extension, or application/octet-stream when the extension is
// unknown or names a type a browser would run or render as a document (HTML,
// XHTML, SVG, XML, XSL, script, Flash), so a direct upload cannot publish
// active content under the public base.
func (s *Client) GenerateUploadAuth(ctx context.Context, req storage.UploadAuthRequest) (storage.UploadAuthResult, error) {
	fileName, err := storage.BuildUploadFileName(req.FileName, s.config.UploadNaming)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key, err := storage.JoinUploadKey(s.config.Dir, req.Dir, fileName)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	key, err = storage.NormalizeKey(key)
	if err != nil {
		return storage.UploadAuthResult{}, err
	}

	contentType := uploadContentType(key)
	preSignedURL, err := s.client.PresignHeader(ctx,
		http.MethodPut,
		s.config.Bucket,
		key,
		storage.ResolveUploadTTL(req.TTL, s.config.UploadTTL, defaultUploadTTL),
		nil,
		http.Header{"Content-Type": {contentType}})
	if err != nil {
		return storage.UploadAuthResult{}, err
	}
	return storage.UploadAuthResult{
		Authorization: storage.UploadAuthorization{
			Type:    storage.UploadAuthorizationTypeURL,
			Value:   preSignedURL.String(),
			Method:  http.MethodPut,
			Headers: map[string]string{"Content-Type": contentType},
		},
		File: storage.UploadFileInfo{
			Key: key,
			URL: s.GenerateURL(key),
		},
	}, nil
}

// UploadFile uploads data from a reader to S3-compatible storage with the specified key.
// The body is buffered up to Config.PartSize, in a buffer that grows with the
// data rather than being allocated up front. A body that ends within that is
// sent with a single PutObject; a longer one as a multipart upload of
// Config.PartSize parts that reuses the buffer, so an upload holds at most one
// part in memory. A failed multipart upload is aborted and leaves any existing
// object in place. Content-Type comes from the key's extension. It returns the
// normalized key.
func (s *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return "", err
	}
	opts := minio.PutObjectOptions{
		ContentType: mime.TypeByExtension(filepath.Ext(key)),
		PartSize:    s.config.PartSize,
	}
	partSize := int(s.config.PartSize)
	buf, n, err := readPart(file, make([]byte, min(initialPartBuffer, partSize)), partSize)
	if err != nil {
		return "", err
	}
	if n == partSize {
		// A full part may or may not be the whole body; one more byte decides.
		var probe [1]byte
		read, err := io.ReadFull(file, probe[:])
		switch {
		case read == 1:
			rest := io.MultiReader(bytes.NewReader(probe[:]), file)
			if err := s.uploadMultipart(ctx, key, buf, n, rest, opts); err != nil {
				return "", err
			}
			return key, nil
		case err != io.EOF:
			return "", err
		}
	}
	if _, err := s.client.PutObject(ctx, s.config.Bucket, key, bytes.NewReader(buf[:n]), int64(n), opts); err != nil {
		return "", err
	}
	return key, nil
}

// uploadMultipart stores the first n bytes of buf followed by rest under key
// as a multipart upload of Config.PartSize parts, reading each further part
// into buf once the previous one is sent. On failure the upload is aborted,
// even when ctx is done, so its parts do not linger.
func (s *Client) uploadMultipart(ctx context.Context, key string, buf []byte, n int, rest io.Reader, opts minio.PutObjectOptions) (err error) {
	core := minio.Core{Client: s.client}
	uploadID, err := core.NewMultipartUpload(ctx, s.config.Bucket, key, opts)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = core.AbortMultipartUpload(context.WithoutCancel(ctx), s.config.Bucket, key, uploadID)
		}
	}()
	var parts []minio.CompletePart
	for number := 1; n > 0; number++ {
		if number > maxUploadParts {
			return fmt.Errorf("s3: upload of %q exceeds %d parts of %d bytes", key, maxUploadParts, s.config.PartSize)
		}
		var uploaded minio.ObjectPart
		uploaded, err = core.PutObjectPart(ctx, s.config.Bucket, key, uploadID, number, bytes.NewReader(buf[:n]), int64(n), minio.PutObjectPartOptions{})
		if err != nil {
			return err
		}
		parts = append(parts, minio.CompletePart{PartNumber: number, ETag: uploaded.ETag})
		if buf, n, err = readPart(rest, buf, int(s.config.PartSize)); err != nil {
			return err
		}
	}
	_, err = core.CompleteMultipartUpload(ctx, s.config.Bucket, key, uploadID, parts, minio.PutObjectOptions{})
	return err
}

// readPart fills buf from r until size bytes are read or r is exhausted, and
// returns the buffer, which may have been replaced by a larger one, with the
// number of bytes read; io.EOF is not an error. The buffer doubles as needed,
// never beyond size, so starting from a small one a short body costs only
// about its own length.
func readPart(r io.Reader, buf []byte, size int) ([]byte, int, error) {
	filled := 0
	for filled < size {
		if filled == len(buf) {
			grown := make([]byte, min(max(2*len(buf), initialPartBuffer), size))
			copy(grown, buf)
			buf = grown
		}
		n, err := r.Read(buf[filled:])
		filled += n
		if err == io.EOF {
			return buf, filled, nil
		}
		if err != nil {
			return buf, filled, err
		}
	}
	return buf, filled, nil
}

// UploadLocalFile uploads an existing local file to S3-compatible storage with the specified key.
// Content-Type comes from the key's extension. It returns the normalized key.
func (s *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return "", err
	}
	info, err := s.client.FPutObject(ctx, s.config.Bucket, key, file, minio.PutObjectOptions{
		ContentType: mime.TypeByExtension(filepath.Ext(key)),
	})
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

// StatFile returns lightweight metadata for a file without downloading its body.
// It implements storage.FileStater by reusing the S3 stat (HEAD) call.
// A missing key fails with storageerr.ErrNotFound.
func (s *Client) StatFile(ctx context.Context, key string) (storage.FileInfo, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return storage.FileInfo{}, err
	}
	info, err := s.client.StatObject(ctx, s.config.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return storage.FileInfo{}, storageerr.ErrNotFound
		}
		return storage.FileInfo{}, err
	}
	return storage.FileInfo{
		MIME: info.ContentType,
		Size: info.Size,
	}, nil
}

// ListFiles enumerates object keys under prefix with cursor-based pagination.
// It implements storage.FileLister on top of the S3 ListObjects API using
// StartAfter as an exclusive cursor. The returned next cursor is the last key
// of the page when more objects remain, otherwise empty. A non-positive limit
// means 1000. Keys not in storage.NormalizeKey form are skipped. It fails with
// ctx.Err() when ctx ends before the listing is exhausted rather than
// returning a truncated page with an empty cursor.
func (s *Client) ListFiles(ctx context.Context, prefix, cursor string, limit int) ([]string, string, error) {
	if limit <= 0 {
		limit = 1000
	}
	listCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := s.client.ListObjects(listCtx, s.config.Bucket, minio.ListObjectsOptions{
		// A prefix is not a key: an empty prefix means "list everything", so it
		// is only stripped of a leading separator rather than normalized.
		Prefix:     strings.TrimPrefix(prefix, "/"),
		StartAfter: strings.TrimPrefix(cursor, "/"),
		Recursive:  true,
		// One key past the page is enough to tell whether more remain.
		MaxKeys: min(limit, maxListKeys-1) + 1,
	})
	keys := make([]string, 0, min(limit, maxListKeys))
	next := ""
	for object := range ch {
		if object.Err != nil {
			return nil, "", object.Err
		}
		if !isAddressableKey(object.Key) {
			continue
		}
		if len(keys) >= limit {
			// One object beyond the requested page means more remain; stop the
			// listing early and surface a resume cursor.
			next = keys[len(keys)-1]
			cancel()
			break
		}
		keys = append(keys, object.Key)
	}
	// Drain any buffered entries so the ListObjects goroutine can exit cleanly
	// after an early cancel.
	for range ch {
	}
	// minio-go ends the channel without an error object when ctx is cancelled
	// between pages. An empty cursor means "end of list", so a listing cut short
	// must not be reported as complete.
	if next == "" {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
	}
	return keys, next, nil
}

// IsFileExists checks whether a file exists in the S3-compatible storage bucket.
// A missing key reports (false, nil).
func (s *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return false, err
	}
	_, err = s.client.StatObject(ctx, s.config.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DownloadFile retrieves a file from S3-compatible storage with a single GET.
// Returns the body reader (the caller must close it), the stored content type,
// and the content size. A missing key fails with storageerr.ErrNotFound.
func (s *Client) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return storage.DownloadResult{}, err
	}
	// A single GET supplies both the body and its metadata. minio's lazy
	// Object would HEAD first and then GET with If-Match, so a concurrent
	// overwrite would fail the read mid-stream with 412.
	body, info, _, err := minio.Core{Client: s.client}.GetObject(ctx, s.config.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return storage.DownloadResult{}, storageerr.ErrNotFound
		}
		return storage.DownloadResult{}, err
	}
	return storage.DownloadResult{
		Reader: body,
		MIME:   info.ContentType,
		Size:   info.Size,
	}, nil
}

// DeleteFile removes a file from the S3-compatible storage bucket.
// S3 treats deleting a missing key as success.
func (s *Client) DeleteFile(ctx context.Context, key string) error {
	key, err := storage.NormalizeKey(key)
	if err != nil {
		return err
	}
	err = s.client.RemoveObject(ctx, s.config.Bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

// MoveFile relocates a file from source to destination key within the S3 bucket.
// It performs a copy operation followed by deletion of the source file, so it
// is not atomic: if the delete fails, the object exists at both keys. A move
// onto itself is a no-op when the source exists. Errors follow CopyFile.
func (s *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey, err := storage.NormalizeKey(sourceKey)
	if err != nil {
		return err
	}
	destinationKey, err = storage.NormalizeKey(destinationKey)
	if err != nil {
		return err
	}
	// A move onto itself is a no-op, and must not run copy-then-delete: the
	// delete would remove the very object the copy just wrote, reporting
	// success while destroying the object.
	if sourceKey == destinationKey {
		exists, err := s.IsFileExists(ctx, sourceKey)
		if err != nil {
			return err
		}
		if !exists {
			return storageerr.ErrNotFound
		}
		return nil
	}
	err = s.CopyFile(ctx, sourceKey, destinationKey, overwrite)
	if err != nil {
		return err
	}
	err = s.client.RemoveObject(ctx, s.config.Bucket, sourceKey, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

// CopyFile duplicates a file from source to destination key within the S3 bucket.
// A missing source fails with storageerr.ErrNotFound, even when the
// destination exists, and an existing destination with overwrite false fails
// with storageerr.ErrDestExists. Copying a key onto itself leaves the object
// unchanged and makes no copy request.
//
// The source is stat'ed first. A source of up to 5 GiB is copied with one
// CopyObject request; a larger one, which CopyObject refuses, with a multipart
// copy of 5 GiB parts that carries over its Content-Type and metadata.
//
// When overwrite is false the destination is checked with a separate stat call
// before copying. The S3 CopyObject API has no portable conditional (there is
// no If-None-Match on the copy destination), so this guard is best-effort only
// and is NOT a concurrency-safe guarantee: a racing writer between the stat and
// the copy can still be clobbered. MoveFile inherits the same caveat.
func (s *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourceKey, err := storage.NormalizeKey(sourceKey)
	if err != nil {
		return err
	}
	destinationKey, err = storage.NormalizeKey(destinationKey)
	if err != nil {
		return err
	}
	source, err := s.client.StatObject(ctx, s.config.Bucket, sourceKey, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKeyError(err) {
			return storageerr.ErrNotFound
		}
		return err
	}
	// S3 rejects a copy of an object onto itself that changes no metadata, so
	// the result is decided without one: the destination is the source.
	if sourceKey == destinationKey {
		if !overwrite {
			return storageerr.ErrDestExists
		}
		return nil
	}
	if !overwrite {
		_, err := s.client.StatObject(ctx, s.config.Bucket, destinationKey, minio.StatObjectOptions{})
		if err == nil {
			return storageerr.ErrDestExists
		}
		if !isNoSuchKeyError(err) {
			return err
		}
	}
	if source.Size > maxCopyObjectSize {
		err = s.copyMultipart(ctx, sourceKey, source, destinationKey)
	} else {
		_, err = s.client.CopyObject(ctx, minio.CopyDestOptions{
			Bucket: s.config.Bucket,
			Object: destinationKey,
		}, minio.CopySrcOptions{
			Bucket: s.config.Bucket,
			Object: sourceKey,
		})
	}
	if err != nil {
		if isNoSuchKeyError(err) {
			return storageerr.ErrNotFound
		}
		return err
	}
	return nil
}

// copyMultipart copies sourceKey, described by source, to destinationKey with
// UploadPartCopy requests of at most maxCopyObjectSize bytes. A multipart
// upload takes its headers from its initiation rather than from the source,
// so Content-Type, the other standard headers and user metadata are passed
// there. Every part is pinned to the source's ETag, so a source replaced
// mid-copy fails the copy instead of mixing versions. On failure the upload
// is aborted, even when ctx is done.
func (s *Client) copyMultipart(ctx context.Context, sourceKey string, source minio.ObjectInfo, destinationKey string) (err error) {
	core := minio.Core{Client: s.client}
	uploadID, err := core.NewMultipartUpload(ctx, s.config.Bucket, destinationKey, minio.PutObjectOptions{
		ContentType:        source.ContentType,
		ContentEncoding:    source.Metadata.Get("Content-Encoding"),
		ContentDisposition: source.Metadata.Get("Content-Disposition"),
		ContentLanguage:    source.Metadata.Get("Content-Language"),
		CacheControl:       source.Metadata.Get("Cache-Control"),
		UserMetadata:       source.UserMetadata,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = core.AbortMultipartUpload(context.WithoutCancel(ctx), s.config.Bucket, destinationKey, uploadID)
		}
	}()
	ifMatch := map[string]string{"x-amz-copy-source-if-match": source.ETag}
	var parts []minio.CompletePart
	for number, offset := 1, int64(0); offset < source.Size; number, offset = number+1, offset+maxCopyObjectSize {
		var part minio.CompletePart
		part, err = core.CopyObjectPart(ctx, s.config.Bucket, sourceKey, s.config.Bucket, destinationKey, uploadID,
			number, offset, min(maxCopyObjectSize, source.Size-offset), ifMatch)
		if err != nil {
			return err
		}
		parts = append(parts, part)
	}
	_, err = core.CompleteMultipartUpload(ctx, s.config.Bucket, destinationKey, uploadID, parts, minio.PutObjectOptions{})
	return err
}

// uploadContentType is the Content-Type signed into a presigned upload of key:
// the type of its extension, or application/octet-stream when that is unknown
// or active content.
func uploadContentType(key string) string {
	const fallback = "application/octet-stream"
	contentType := mime.TypeByExtension(filepath.Ext(key))
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || isActiveContentType(mediaType) {
		return fallback
	}
	return contentType
}

// isActiveContentType reports whether a browser navigating to a response of
// mediaType (lowercase, without parameters) would run script or render it as
// a document that can.
func isActiveContentType(mediaType string) bool {
	_, subtype, _ := strings.Cut(mediaType, "/")
	return strings.Contains(subtype, "html") ||
		strings.Contains(subtype, "script") ||
		strings.Contains(subtype, "svg") ||
		strings.Contains(subtype, "xsl") ||
		subtype == "xml" || strings.HasPrefix(subtype, "xml-") || strings.HasSuffix(subtype, "+xml") ||
		subtype == "x-shockwave-flash"
}

// isAddressableKey reports whether the rest of the API can reach key: it must
// already be in storage.NormalizeKey form, which every method applies.
func isAddressableKey(key string) bool {
	normalized, err := storage.NormalizeKey(key)
	return err == nil && normalized == key
}

func isNoSuchKeyError(err error) bool {
	return minio.ToErrorResponse(err).Code == minio.NoSuchKey
}
