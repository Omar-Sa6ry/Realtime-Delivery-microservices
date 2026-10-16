package testutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/ports"
)

var _ ports.ObjectStorage = (*MemStorage)(nil)

type memObject struct {
	Data        []byte
	ContentType string
	ETag        string
	Modified    time.Time
}

// MemStorage is a fully in-memory ObjectStorage implementation.
//
// Error injection: call SetError with one of the operation names below to make
// the corresponding method fail until ClearErrors is called.
//
//	"CreateMultipartUpload", "GeneratePresignedParts", "GeneratePresignedGET",
//	"GeneratePresignedGETWithRange", "CompleteMultipartUpload",
//	"AbortMultipartUpload", "HeadObject", "DeleteObject", "DeleteObjects",
//	"GetObject", "GetObjectWithRange", "PutObject", "ListObjectsWithPrefix"
type MemStorage struct {
	mu sync.Mutex

	objects    map[string]*memObject
	uploads    map[string]string // s3 upload id -> object key
	uploadSeq  int
	errs       map[string]error
	calls      []string
	headSizes  map[string]int64
	listResult map[string][]string
	presigned  []PresignedCall
}

// PresignedCall records a presigned URL generation request.
type PresignedCall struct {
	Op          string
	ObjectKey   string
	RangeHeader string
	Expiry      time.Duration
	TotalParts  int
}

// NewMemStorage returns an empty storage fake.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		objects:    make(map[string]*memObject),
		uploads:    make(map[string]string),
		errs:       make(map[string]error),
		headSizes:  make(map[string]int64),
		listResult: make(map[string][]string),
	}
}

// SetError makes the named operation fail with err.
func (s *MemStorage) SetError(op string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs[op] = err
}

// ClearErrors removes every injected error.
func (s *MemStorage) ClearErrors() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = make(map[string]error)
}

// Calls returns the recorded operation names (optionally suffixed with ":key").
func (s *MemStorage) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

// PresignedCalls returns the recorded presign requests.
func (s *MemStorage) PresignedCalls() []PresignedCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]PresignedCall(nil), s.presigned...)
}

func (s *MemStorage) fail(op, key string) error {
	s.calls = append(s.calls, op+":"+key)
	return s.errs[op]
}

// SetObject stores raw bytes for key.
func (s *MemStorage) SetObject(key string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = &memObject{
		Data:        append([]byte(nil), data...),
		ContentType: "application/octet-stream",
		ETag:        etagFor(data),
		Modified:    time.Now().UTC(),
	}
}

// SetObjectWithContentType stores raw bytes for key with a content type.
func (s *MemStorage) SetObjectWithContentType(key, contentType string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = &memObject{
		Data:        append([]byte(nil), data...),
		ContentType: contentType,
		ETag:        etagFor(data),
		Modified:    time.Now().UTC(),
	}
}

// Object returns a copy of the stored bytes for key.
func (s *MemStorage) Object(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objects[key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), o.Data...), true
}

// ObjectCount returns the number of stored objects.
func (s *MemStorage) ObjectCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.objects)
}

// SetHeadSize forces HeadObject to report size for key (useful when the object
// body is not materialised in the fake).
func (s *MemStorage) SetHeadSize(key string, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.headSizes[key] = size
}

// SetListResult forces ListObjectsWithPrefix to return keys for prefix.
func (s *MemStorage) SetListResult(prefix string, keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listResult[prefix] = keys
}

func etagFor(data []byte) string {
	return fmt.Sprintf("\"%x\"", len(data))
}

func (s *MemStorage) CreateMultipartUpload(ctx context.Context, objectKey, contentType string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail("CreateMultipartUpload", objectKey); err != nil {
		return "", err
	}
	s.uploadSeq++
	id := fmt.Sprintf("s3-upload-%d", s.uploadSeq)
	s.uploads[id] = objectKey
	if _, ok := s.objects[objectKey]; !ok {
		s.objects[objectKey] = &memObject{Data: nil, ContentType: contentType, ETag: etagFor(nil), Modified: time.Now().UTC()}
	}
	return id, nil
}

func (s *MemStorage) GeneratePresignedParts(ctx context.Context, objectKey, s3UploadID string, totalParts int, partSize int64, expiry time.Duration) ([]ports.PresignedPart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presigned = append(s.presigned, PresignedCall{
		Op: "GeneratePresignedParts", ObjectKey: objectKey, Expiry: expiry, TotalParts: totalParts,
	})
	if err := s.fail("GeneratePresignedParts", objectKey); err != nil {
		return nil, err
	}
	out := make([]ports.PresignedPart, 0, totalParts)
	for i := 1; i <= totalParts; i++ {
		out = append(out, ports.PresignedPart{
			PartNumber:   i,
			PresignedURL: fmt.Sprintf("https://storage.test/%s?partNumber=%d&uploadId=%s&exp=%d", objectKey, i, s3UploadID, expiry),
		})
	}
	return out, nil
}

func (s *MemStorage) GeneratePresignedGET(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	return s.GeneratePresignedGETWithRange(ctx, objectKey, "", expiry)
}

func (s *MemStorage) GeneratePresignedGETWithRange(ctx context.Context, objectKey, rangeHeader string, expiry time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presigned = append(s.presigned, PresignedCall{
		Op: "GeneratePresignedGETWithRange", ObjectKey: objectKey, RangeHeader: rangeHeader, Expiry: expiry,
	})
	if err := s.fail("GeneratePresignedGETWithRange", objectKey); err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://storage.test/%s?X-Amz-Expires=%d", objectKey, int(expiry.Seconds()))
	if rangeHeader != "" {
		url += "&range=" + rangeHeader
	}
	return url, nil
}

func (s *MemStorage) CompleteMultipartUpload(ctx context.Context, objectKey, s3UploadID string, parts []domain.UploadPart) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail("CompleteMultipartUpload", objectKey); err != nil {
		return err
	}
	if _, ok := s.uploads[s3UploadID]; !ok {
		return fmt.Errorf("no such upload %q", s3UploadID)
	}
	delete(s.uploads, s3UploadID)
	return nil
}

func (s *MemStorage) AbortMultipartUpload(ctx context.Context, objectKey, s3UploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail("AbortMultipartUpload", objectKey); err != nil {
		return err
	}
	delete(s.uploads, s3UploadID)
	delete(s.objects, objectKey)
	return nil
}

func (s *MemStorage) HeadObject(ctx context.Context, objectKey string) (*ports.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail("HeadObject", objectKey); err != nil {
		return nil, err
	}
	if size, ok := s.headSizes[objectKey]; ok {
		return &ports.ObjectInfo{
			Size:         size,
			ETag:         fmt.Sprintf("\"%d\"", size),
			ContentType:  "application/octet-stream",
			LastModified: time.Now().UTC(),
		}, nil
	}
	o, ok := s.objects[objectKey]
	if !ok {
		return nil, fmt.Errorf("object %q not found", objectKey)
	}
	return &ports.ObjectInfo{
		Size:         int64(len(o.Data)),
		ETag:         o.ETag,
		ContentType:  o.ContentType,
		LastModified: o.Modified,
	}, nil
}

func (s *MemStorage) DeleteObject(ctx context.Context, objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail("DeleteObject", objectKey); err != nil {
		return err
	}
	delete(s.objects, objectKey)
	return nil
}

func (s *MemStorage) DeleteObjects(ctx context.Context, objectKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail("DeleteObjects", strings.Join(objectKeys, ",")); err != nil {
		return err
	}
	for _, k := range objectKeys {
		delete(s.objects, k)
	}
	return nil
}

func (s *MemStorage) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return s.GetObjectWithRange(ctx, objectKey, "")
}

func (s *MemStorage) GetObjectWithRange(ctx context.Context, objectKey, rangeHeader string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "GetObject:"+objectKey)
	if err := s.errs["GetObject"]; err != nil {
		return nil, err
	}
	if err := s.errs["GetObjectWithRange"]; err != nil {
		return nil, err
	}
	o, ok := s.objects[objectKey]
	if !ok {
		return nil, fmt.Errorf("object %q not found", objectKey)
	}
	data := o.Data
	if rangeHeader != "" {
		if start, end, ok := parseRange(rangeHeader, int64(len(data))); ok {
			data = data[start:end]
		}
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func parseRange(header string, size int64) (int, int, bool) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(header, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	var start int64
	if parts[0] != "" {
		if _, err := fmt.Sscanf(parts[0], "%d", &start); err != nil {
			return 0, 0, false
		}
	}
	end := size - 1
	if parts[1] != "" {
		if _, err := fmt.Sscanf(parts[1], "%d", &end); err != nil {
			return 0, 0, false
		}
	}
	if start < 0 || end >= size || start > end {
		return 0, 0, false
	}
	return int(start), int(end) + 1, true
}

func (s *MemStorage) PutObject(ctx context.Context, objectKey, contentType string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail("PutObject", objectKey); err != nil {
		return err
	}
	s.objects[objectKey] = &memObject{
		Data:        append([]byte(nil), data...),
		ContentType: contentType,
		ETag:        etagFor(data),
		Modified:    time.Now().UTC(),
	}
	delete(s.headSizes, objectKey)
	return nil
}

func (s *MemStorage) ListObjectsWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail("ListObjectsWithPrefix", prefix); err != nil {
		return nil, err
	}
	if forced, ok := s.listResult[prefix]; ok {
		return append([]string(nil), forced...), nil
	}
	var keys []string
	for k := range s.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
