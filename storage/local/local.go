package local

import (
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
)

// Config holds the configuration for local file storage operations.
type Config struct {
	// RootDir is the directory every key resolves under. Keys are confined to it
	// lexically, which stops traversal through "..", but symlinks already present
	// inside the directory are followed and are not checked for escaping it: the
	// contents of RootDir are assumed to be trusted and managed by this process.
	RootDir string `json:"root_dir" yaml:"root_dir"`
}

const (
	// defaultFileMode is applied to newly created files. Writes go through
	// os.CreateTemp, which opens at 0o600, so the mode is set explicitly to stay
	// consistent with the os.Create-based writes this replaced. Overwrites keep
	// the existing file's permissions instead.
	defaultFileMode os.FileMode = 0o644
	// tmpFilePrefix marks the in-progress temporary files written by
	// writeFileAtomic. ListFiles hides them so a crash mid-write cannot leave
	// behind something that looks like a stored key.
	tmpFilePrefix = ".sphere-tmp-"
)

// writeFileAtomic streams src into destPath through a temporary file in the
// same directory followed by a rename, so a failure partway through can never
// leave the destination missing or half written; an existing destination is
// replaced only once the new contents are durable. Any overwrite policy is the
// caller's responsibility, as the rename replaces whatever is already there.
func writeFileAtomic(destPath string, src io.Reader) error {
	mode := defaultFileMode
	if stat, err := os.Stat(destPath); err == nil {
		// Preserve the permissions of the file being replaced.
		mode = stat.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	// Creating the temporary file in destPath's directory keeps the rename below
	// on one filesystem, where it is atomic and cannot fail with EXDEV.
	tmp, err := os.CreateTemp(filepath.Dir(destPath), tmpFilePrefix+"*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, err = io.Copy(tmp, src)
	if err == nil {
		err = tmp.Sync()
	}
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if closeErr := tmp.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpPath, destPath)
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// Client provides local filesystem storage operations.
// It implements the Storage interface for file operations on the local filesystem.
type Client struct {
	config Config
}

// NewClient creates a new local storage client with the provided configuration.
// It validates the root directory and creates it if it doesn't exist.
// Returns an error if the root directory cannot be created or is invalid.
func NewClient(conf Config) (*Client, error) {
	if conf.RootDir == "" {
		return nil, errors.New("root_dir is required")
	}
	err := os.MkdirAll(conf.RootDir, 0o750)
	if err != nil {
		return nil, err
	}
	return &Client{
		config: conf,
	}, nil
}

// fixFilePath resolves and validates file paths to prevent directory traversal attacks.
// It ensures that all file operations stay within the configured root directory.
func (c *Client) fixFilePath(key string) (string, error) {
	rootDir, err := filepath.Abs(c.config.RootDir)
	if err != nil {
		return "", err
	}
	filePath, err := filepath.Abs(filepath.Join(c.config.RootDir, filepath.Clean(key)))
	if err != nil {
		return "", err
	}
	rootDir = filepath.Clean(rootDir)
	filePath = filepath.Clean(filePath)
	rel, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		return "", err
	}
	// rel == "." means the key resolves to the root directory itself (e.g. an
	// empty key, ".", or "/"); such keys are not valid file targets.
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", storageerr.ErrorFileNameInvalid
	}
	return filePath, nil
}

// UploadFile uploads data from a reader to the local filesystem with the specified key.
// It creates the necessary directory structure and writes the file content.
// The write is atomic: a failure leaves any previous content at the key intact
// and never publishes a partially written file.
func (c *Client) UploadFile(ctx context.Context, file io.Reader, key string) (string, error) {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(filepath.Dir(filePath), 0o750)
	if err != nil {
		return "", err
	}
	// Bail out early if the caller has already cancelled. The streaming copy
	// below is not itself interruptible via ctx, so cancellation issued mid
	// copy is not enforced; the write is atomic either way.
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = writeFileAtomic(filePath, file); err != nil {
		return "", err
	}
	return key, nil
}

// UploadLocalFile uploads an existing local file to the storage with the specified key.
// This is useful for moving files within the local filesystem storage.
func (c *Client) UploadLocalFile(ctx context.Context, file string, key string) (string, error) {
	raw, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = raw.Close()
	}()
	return c.UploadFile(ctx, raw, key)
}

// IsFileExists checks whether a file exists in the local filesystem storage.
func (c *Client) IsFileExists(ctx context.Context, key string) (bool, error) {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return false, err
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	// A directory is never a retrievable file; report it as non-existent.
	if stat.IsDir() {
		return false, nil
	}
	return true, nil
}

// StatFile returns lightweight metadata for a file without opening its contents.
// It implements storage.FileStater by reusing the filesystem stat call.
func (c *Client) StatFile(ctx context.Context, key string) (storage.FileInfo, error) {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return storage.FileInfo{}, err
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.FileInfo{}, storageerr.ErrorNotFound
		}
		return storage.FileInfo{}, err
	}
	// A directory is never a retrievable file; report it as missing.
	if stat.IsDir() {
		return storage.FileInfo{}, storageerr.ErrorNotFound
	}
	return storage.FileInfo{
		MIME: mime.TypeByExtension(filepath.Ext(key)),
		Size: stat.Size(),
	}, nil
}

// ListFiles enumerates stored keys under prefix with cursor-based pagination.
// It implements storage.FileLister by walking the root directory; keys use
// forward slashes and are returned in lexical order. The cursor is exclusive:
// pass the previous next value to resume after the last returned key.
//
// Every call walks and sorts the whole root directory before applying the
// cursor, so paging a large tree costs O(total files) per page. That is
// acceptable for the local driver's development and small-deployment use, but
// it does not scale the way the object-store drivers' native listings do.
// Symlinks are skipped: the walk only reports regular files, even though a
// symlink to one inside the root remains readable through DownloadFile.
func (c *Client) ListFiles(ctx context.Context, prefix, cursor string, limit int) ([]string, string, error) {
	if limit <= 0 {
		limit = 1000
	}
	prefix = strings.TrimPrefix(prefix, "/")
	cursor = strings.TrimPrefix(cursor, "/")
	rootDir, err := filepath.Abs(c.config.RootDir)
	if err != nil {
		return nil, "", err
	}
	rootDir = filepath.Clean(rootDir)

	var all []string
	walkErr := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		// An in-progress (or orphaned) atomic write is not a stored key.
		if strings.HasPrefix(d.Name(), tmpFilePrefix) {
			return nil
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		key := filepath.ToSlash(rel)
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}
		all = append(all, key)
		return nil
	})
	if walkErr != nil {
		return nil, "", walkErr
	}
	sort.Strings(all)

	keys := make([]string, 0, limit)
	next := ""
	for _, key := range all {
		if cursor != "" && key <= cursor {
			continue
		}
		if len(keys) >= limit {
			// At least one key remains beyond the page; expose a resume cursor.
			next = keys[len(keys)-1]
			break
		}
		keys = append(keys, key)
	}
	return keys, next, nil
}

// DownloadFile retrieves a file from local filesystem storage.
// Returns the file reader, MIME type based on file extension, and file size.
func (c *Client) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return storage.DownloadResult{}, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return storage.DownloadResult{}, storageerr.ErrorNotFound
		}
		return storage.DownloadResult{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return storage.DownloadResult{}, err
	}
	// Never hand back a directory handle; treat it as a missing file so the
	// caller does not leak the descriptor or hit EISDIR on read.
	if stat.IsDir() {
		_ = file.Close()
		return storage.DownloadResult{}, storageerr.ErrorNotFound
	}
	return storage.DownloadResult{
		Reader: file,
		MIME:   mime.TypeByExtension(filepath.Ext(key)),
		Size:   stat.Size(),
	}, nil
}

// DeleteFile removes a file from the local filesystem storage.
// Deletion is idempotent: a key that holds no regular file reports success.
func (c *Client) DeleteFile(ctx context.Context, key string) error {
	filePath, err := c.fixFilePath(key)
	if err != nil {
		return err
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// Only regular files are valid storage keys. A directory or special file is
	// not addressable as a key (IsFileExists reports it absent), so deleting it
	// is a no-op rather than a recursive removal of a possibly shared subtree.
	if !stat.Mode().IsRegular() {
		return nil
	}
	err = os.Remove(filePath)
	if err != nil {
		// Lost a race with a concurrent deleter; the postcondition still holds.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

// checkOverwrite enforces the overwrite policy for move and copy operations,
// reporting ErrorDistExisted when the destination exists and overwrite is off.
// It deliberately does not remove the destination: both callers finish with a
// rename, which replaces it atomically, so nothing is destroyed before the
// replacement is ready.
func (c *Client) checkOverwrite(path string, overwrite bool) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !overwrite {
		return storageerr.ErrorDistExisted
	}
	return nil
}

// MoveFile relocates a file from source to destination key within local filesystem storage.
// Creates necessary directory structure and handles overwrite logic.
func (c *Client) MoveFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourcePath, err := c.fixFilePath(sourceKey)
	if err != nil {
		return err
	}
	destinationPath, err := c.fixFilePath(destinationKey)
	if err != nil {
		return err
	}
	if e := os.MkdirAll(filepath.Dir(destinationPath), 0o750); e != nil {
		return e
	}
	if stat, e := os.Stat(sourcePath); e != nil {
		if os.IsNotExist(e) {
			return storageerr.ErrorNotFound
		}
		return e
	} else if !stat.Mode().IsRegular() {
		// Never relocate a whole directory (or special file) under a key.
		return storageerr.ErrorNotFound
	}
	if e := c.checkOverwrite(destinationPath, overwrite); e != nil {
		return e
	}
	if e := os.Rename(sourcePath, destinationPath); e != nil {
		return e
	}
	return nil
}

// CopyFile duplicates a file from source to destination key within local filesystem storage.
// Creates necessary directory structure and handles overwrite logic.
// The copy is atomic: the destination is replaced only once the full contents
// are written, so a failure at any point leaves it untouched.
func (c *Client) CopyFile(ctx context.Context, sourceKey string, destinationKey string, overwrite bool) error {
	sourcePath, err := c.fixFilePath(sourceKey)
	if err != nil {
		return err
	}
	destinationPath, err := c.fixFilePath(destinationKey)
	if err != nil {
		return err
	}
	if e := os.MkdirAll(filepath.Dir(destinationPath), 0o750); e != nil {
		return e
	}
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return storageerr.ErrorNotFound
		}
		return err
	}
	defer func() {
		_ = srcFile.Close()
	}()
	srcStat, err := srcFile.Stat()
	if err != nil {
		return err
	}
	// Never duplicate a directory (or special file) as if it were a file key.
	if !srcStat.Mode().IsRegular() {
		return storageerr.ErrorNotFound
	}
	if e := c.checkOverwrite(destinationPath, overwrite); e != nil {
		return e
	}
	return writeFileAtomic(destinationPath, srcFile)
}
