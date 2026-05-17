package localblob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	resourceblob "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/resourceblob"
)

const (
	Kind = "localblob"
)

type Config struct {
	BaseDir string
}

type Store struct {
	baseDir string

	mu    sync.RWMutex
	blobs map[string]resourceblob.BlobInfo
}

func NewStore(cfg Config) (*Store, error) {
	if cfg.BaseDir == "" {
		return nil, resourceblob.ErrStoreNotReady
	}
	if err := os.MkdirAll(cfg.BaseDir, 0o700); err != nil {
		return nil, err
	}
	return &Store{
		baseDir: cfg.BaseDir,
		blobs:   make(map[string]resourceblob.BlobInfo),
	}, nil
}

func (s *Store) Kind() string {
	return Kind
}

func (s *Store) Put(ctx context.Context, req resourceblob.PutRequest) (*resourceblob.StoredBlob, error) {
	if err := validateResourceID(req.ResourceID); err != nil {
		return nil, err
	}
	if req.Source == nil {
		return nil, resourceblob.ErrStoreNotReady
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.blobs[req.ResourceID]; ok {
		return nil, resourceblob.ErrAlreadyExists
	}
	if _, err := os.Stat(s.blobPath(req.ResourceID)); err == nil {
		return nil, resourceblob.ErrAlreadyExists
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	tmpPath := s.tmpPath(req.ResourceID)
	blobPath := s.blobPath(req.ResourceID)
	if err := os.MkdirAll(s.baseDir, 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	cleanupTmp := true
	defer func() {
		_ = tmp.Close()
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	size, err := copyWithContext(ctx, tmp, req.Source, hasher)
	if err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, blobPath); err != nil {
		return nil, err
	}
	cleanupTmp = false

	now := time.Now().UTC()
	info := resourceblob.BlobInfo{
		ResourceID:   req.ResourceID,
		StoreKind:    s.Kind(),
		StoreBlobID:  req.ResourceID,
		Name:         req.Name,
		MimeType:     req.MimeType,
		SizeBytes:    size,
		ContentHash:  "sha256:" + hex.EncodeToString(hasher.Sum(nil)),
		CreatedAt:    now,
		MetadataJSON: append([]byte(nil), req.MetadataJSON...),
	}
	s.blobs[req.ResourceID] = cloneInfo(info)
	stored := storedFromInfo(info)
	return &stored, nil
}

func (s *Store) Open(_ context.Context, resourceID string) (io.ReadCloser, *resourceblob.BlobInfo, error) {
	if err := validateResourceID(resourceID); err != nil {
		return nil, nil, err
	}
	s.mu.RLock()
	info, ok := s.blobs[resourceID]
	s.mu.RUnlock()
	if !ok {
		return nil, nil, resourceblob.ErrNotFound
	}
	file, err := os.Open(s.blobPath(resourceID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, resourceblob.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	cloned := cloneInfo(info)
	return file, &cloned, nil
}

func (s *Store) Stat(_ context.Context, resourceID string) (*resourceblob.BlobInfo, error) {
	if err := validateResourceID(resourceID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.blobs[resourceID]
	if !ok {
		return nil, resourceblob.ErrNotFound
	}
	cloned := cloneInfo(info)
	return &cloned, nil
}

func (s *Store) Delete(_ context.Context, resourceID string) error {
	if err := validateResourceID(resourceID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.blobs[resourceID]; !ok {
		return resourceblob.ErrNotFound
	}
	if err := os.Remove(s.blobPath(resourceID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(s.blobs, resourceID)
	return nil
}

func (s *Store) blobPath(resourceID string) string {
	return filepath.Join(s.baseDir, resourceID+".blob")
}

func (s *Store) tmpPath(resourceID string) string {
	return filepath.Join(s.baseDir, resourceID+".tmp")
}

func validateResourceID(resourceID string) error {
	if resourceID == "" ||
		strings.ContainsRune(resourceID, 0) ||
		strings.Contains(resourceID, "/") ||
		strings.Contains(resourceID, `\`) ||
		resourceID == "." ||
		resourceID == ".." ||
		strings.Contains(resourceID, "..") {
		return resourceblob.ErrInvalidResourceID
	}
	return nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader, hasher hash.Hash) (int64, error) {
	var written int64
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			chunk := buf[:nr]
			nw, ew := dst.Write(chunk)
			if nw > 0 {
				_, _ = hasher.Write(chunk[:nw])
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}

func storedFromInfo(info resourceblob.BlobInfo) resourceblob.StoredBlob {
	return resourceblob.StoredBlob{
		ResourceID:   info.ResourceID,
		StoreKind:    info.StoreKind,
		StoreBlobID:  info.StoreBlobID,
		Name:         info.Name,
		MimeType:     info.MimeType,
		SizeBytes:    info.SizeBytes,
		ContentHash:  info.ContentHash,
		CreatedAt:    info.CreatedAt,
		MetadataJSON: append([]byte(nil), info.MetadataJSON...),
	}
}

func cloneInfo(info resourceblob.BlobInfo) resourceblob.BlobInfo {
	info.MetadataJSON = append([]byte(nil), info.MetadataJSON...)
	return info
}

func (s *Store) String() string {
	return fmt.Sprintf("%s:%s", Kind, s.baseDir)
}
