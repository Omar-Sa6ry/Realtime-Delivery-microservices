package testutil

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// S3Error is an error the fake S3 endpoint can return.
type S3Error struct {
	Code    string
	Message string
	Status  int
}

func (e *S3Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// S3Request records one request received by the fake S3 endpoint.
type S3Request struct {
	Op     string
	Method string
	Path   string
	Query  string
	Body   []byte
}

type fakeS3Object struct {
	Data        []byte
	ContentType string
	ETag        string
	Modified    time.Time
}

// FakeS3 is a minimal in-memory S3 endpoint compatible with the AWS SDK v2
// path-style addressing used by the media-service adapter.
type FakeS3 struct {
	server *httptest.Server

	mu        sync.Mutex
	objects   map[string]*fakeS3Object
	uploads   map[string]string // upload id -> object key
	uploadSeq int
	errs      map[string]error
	requests  []S3Request
}

// NewFakeS3 starts the fake endpoint and closes it when the test ends.
func NewFakeS3(t *testing.T) *FakeS3 {
	t.Helper()
	f := &FakeS3{
		objects: make(map[string]*fakeS3Object),
		uploads: make(map[string]string),
		errs:    make(map[string]error),
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

// Close shuts the endpoint down.
func (f *FakeS3) Close() { f.server.Close() }

// Endpoint returns the base URL of the fake endpoint.
func (f *FakeS3) Endpoint() string { return f.server.URL }

// SetError makes the named operation fail (e.g. "PutObject", "HeadObject").
func (f *FakeS3) SetError(op string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs[op] = err
}

// ClearErrors clears every injected error.
func (f *FakeS3) ClearErrors() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = make(map[string]error)
}

// Requests returns the recorded requests.
func (f *FakeS3) Requests() []S3Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]S3Request(nil), f.requests...)
}

// OpCounts returns how many times each operation was invoked.
func (f *FakeS3) OpCounts() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int)
	for _, r := range f.requests {
		out[r.Op]++
	}
	return out
}

// PutObject seeds an object into the fake bucket.
func (f *FakeS3) PutObject(key, contentType string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = &fakeS3Object{
		Data:        append([]byte(nil), data...),
		ContentType: contentType,
		ETag:        fmt.Sprintf("\"%x\"", len(data)),
		Modified:    time.Now().UTC(),
	}
}

// GetObject returns a stored object.
func (f *FakeS3) GetObject(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), o.Data...), true
}

// HasMultipartUpload reports whether uploadID is still open.
func (f *FakeS3) HasMultipartUpload(uploadID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.uploads[uploadID]
	return ok
}

// OpenUploads returns the number of unfinished multipart uploads.
func (f *FakeS3) OpenUploads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.uploads)
}

func (f *FakeS3) classify(r *http.Request) string {
	q := r.URL.Query()
	switch {
	case r.Method == http.MethodPost && q.Has("uploads"):
		return "CreateMultipartUpload"
	case q.Has("uploadId") && r.Method == http.MethodPost:
		return "CompleteMultipartUpload"
	case q.Has("uploadId") && r.Method == http.MethodDelete:
		return "AbortMultipartUpload"
	case q.Has("uploadId") && r.Method == http.MethodPut:
		return "UploadPart"
	case r.Method == http.MethodPost && q.Has("delete"):
		return "DeleteObjects"
	case r.Method == http.MethodGet && q.Has("list-type"):
		return "ListObjectsV2"
	case r.Method == http.MethodPut && q.Has("publicAccessBlock"):
		return "PutPublicAccessBlock"
	case r.Method == http.MethodPut && q.Has("lifecycle"):
		return "PutBucketLifecycle"
	case r.Method == http.MethodPut && q.Has("cors"):
		return "PutBucketCors"
	case r.Method == http.MethodHead:
		return "HeadObject"
	case r.Method == http.MethodDelete:
		return "DeleteObject"
	case r.Method == http.MethodGet:
		return "GetObject"
	case r.Method == http.MethodPut && objectKeyOf(r.URL.Path) == "":
		return "CreateBucket"
	case r.Method == http.MethodPut:
		return "PutObject"
	case r.Method == http.MethodPost:
		return "PostObject"
	default:
		return "Unknown"
	}
}

// objectKeyOf extracts the object key from a path-style URL: /bucket/key...
func objectKeyOf(path string) string {
	p := strings.TrimPrefix(path, "/")
	idx := strings.Index(p, "/")
	if idx < 0 {
		return ""
	}
	return p[idx+1:]
}

func bucketOf(path string) string {
	p := strings.TrimPrefix(path, "/")
	if idx := strings.Index(p, "/"); idx >= 0 {
		return p[:idx]
	}
	return p
}

func (f *FakeS3) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	op := f.classify(r)

	f.mu.Lock()
	f.requests = append(f.requests, S3Request{
		Op: op, Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body,
	})
	injected := f.errs[op]
	f.mu.Unlock()

	if injected != nil {
		se, ok := injected.(*S3Error)
		if !ok {
			se = &S3Error{Code: "InternalError", Message: injected.Error(), Status: 500}
		}
		writeS3Error(w, se, r)
		return
	}

	switch op {
	case "CreateMultipartUpload":
		f.mu.Lock()
		f.uploadSeq++
		id := fmt.Sprintf("upload-%d", f.uploadSeq)
		f.uploads[id] = objectKeyOf(r.URL.Path)
		if _, ok := f.objects[objectKeyOf(r.URL.Path)]; !ok {
			f.objects[objectKeyOf(r.URL.Path)] = &fakeS3Object{ETag: "\"0\"", Modified: time.Now().UTC()}
		}
		f.mu.Unlock()
		writeXML(w, 200, fmt.Sprintf(
			`<?xml version="1.0" encoding="UTF-8"?>
<InitiateMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Bucket>%s</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>`,
			bucketOf(r.URL.Path), objectKeyOf(r.URL.Path), id))

	case "UploadPart":
		w.Header().Set("ETag", fmt.Sprintf("\"part-%s\"", r.URL.Query().Get("partNumber")))
		w.WriteHeader(200)

	case "CompleteMultipartUpload":
		uploadID := r.URL.Query().Get("uploadId")
		f.mu.Lock()
		key, ok := f.uploads[uploadID]
		if ok {
			delete(f.uploads, uploadID)
		}
		f.mu.Unlock()
		if !ok {
			writeS3Error(w, &S3Error{Code: "NoSuchUpload", Message: "upload not found", Status: 404}, r)
			return
		}
		etag := fmt.Sprintf("\"%x\"", len(key))
		writeXML(w, 200, fmt.Sprintf(
			`<?xml version="1.0" encoding="UTF-8"?>
<CompleteMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Location>%s</Location><Bucket>%s</Bucket><Key>%s</Key><ETag>%s</ETag></CompleteMultipartUploadResult>`,
			"http://fake/"+r.URL.Path, bucketOf(r.URL.Path), key, etag))

	case "AbortMultipartUpload":
		f.mu.Lock()
		delete(f.uploads, r.URL.Query().Get("uploadId"))
		f.mu.Unlock()
		w.WriteHeader(204)

	case "HeadObject":
		f.mu.Lock()
		o, exists := f.objects[objectKeyOf(r.URL.Path)]
		f.mu.Unlock()
		if !exists {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(o.Data)))
		w.Header().Set("ETag", o.ETag)
		if o.ContentType != "" {
			w.Header().Set("Content-Type", o.ContentType)
		}
		w.Header().Set("Last-Modified", o.Modified.UTC().Format(http.TimeFormat))
		w.WriteHeader(200)

	case "GetObject":
		f.mu.Lock()
		o, exists := f.objects[objectKeyOf(r.URL.Path)]
		f.mu.Unlock()
		if !exists {
			writeS3Error(w, &S3Error{Code: "NoSuchKey", Message: "no such key", Status: 404}, r)
			return
		}
		data := o.Data
		status := 200
		if rng := r.Header.Get("Range"); rng != "" {
			if start, end, ok := parseS3Range(rng, int64(len(data))); ok {
				data = data[start:end]
				status = 206
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end-1, len(o.Data)))
			}
		}
		w.Header().Set("ETag", o.ETag)
		if o.ContentType != "" {
			w.Header().Set("Content-Type", o.ContentType)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(status)
		_, _ = w.Write(data)

	case "PutObject":
		key := objectKeyOf(r.URL.Path)
		ct := r.Header.Get("Content-Type")
		f.mu.Lock()
		f.objects[key] = &fakeS3Object{
			Data:        append([]byte(nil), body...),
			ContentType: ct,
			ETag:        fmt.Sprintf("\"%x\"", len(body)),
			Modified:    time.Now().UTC(),
		}
		f.mu.Unlock()
		w.Header().Set("ETag", fmt.Sprintf("\"%x\"", len(body)))
		w.WriteHeader(200)

	case "DeleteObject":
		f.mu.Lock()
		delete(f.objects, objectKeyOf(r.URL.Path))
		f.mu.Unlock()
		w.WriteHeader(204)

	case "DeleteObjects":
		f.mu.Lock()
		for _, key := range extractXMLTags(string(body), "Key") {
			delete(f.objects, key)
		}
		f.mu.Unlock()
		writeXML(w, 200, `<?xml version="1.0" encoding="UTF-8"?><DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></DeleteResult>`)

	case "ListObjectsV2":
		prefix := r.URL.Query().Get("prefix")
		var keys []string
		f.mu.Lock()
		for k := range f.objects {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		f.mu.Unlock()
		sort.Strings(keys)
		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		sb.WriteString(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
		sb.WriteString("<Name>" + bucketOf(r.URL.Path) + "</Name>")
		sb.WriteString("<Prefix>" + xmlEscape(prefix) + "</Prefix>")
		sb.WriteString("<KeyCount>" + strconv.Itoa(len(keys)) + "</KeyCount>")
		sb.WriteString("<MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated>")
		for _, k := range keys {
			f.mu.Lock()
			o := f.objects[k]
			f.mu.Unlock()
			size := 0
			etag := `""`
			modified := time.Now().UTC()
			if o != nil {
				size = len(o.Data)
				etag = o.ETag
				modified = o.Modified
			}
			sb.WriteString("<Contents><Key>" + xmlEscape(k) + "</Key><LastModified>" +
				modified.Format("2006-01-02T15:04:05.000Z") + "</LastModified><ETag>" + xmlEscape(etag) +
				"</ETag><Size>" + strconv.Itoa(size) + "</Size><StorageClass>STANDARD</StorageClass></Contents>")
		}
		sb.WriteString("</ListBucketResult>")
		writeXML(w, 200, sb.String())

	case "CreateBucket":
		w.Header().Set("Location", "/"+bucketOf(r.URL.Path))
		w.WriteHeader(200)

	case "PutPublicAccessBlock", "PutBucketLifecycle", "PutBucketCors":
		w.WriteHeader(200)

	case "PostObject":
		writeXML(w, 200, `<?xml version="1.0" encoding="UTF-8"?><PostResponse><Bucket>`+
			xmlEscape(bucketOf(r.URL.Path))+`</Bucket><Key>`+xmlEscape(objectKeyOf(r.URL.Path))+
			`</Key></PostResponse>`)

	default:
		writeS3Error(w, &S3Error{Code: "NotImplemented", Message: op, Status: 501}, r)
	}
}

func parseS3Range(header string, size int64) (int, int, bool) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(header, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 || parts[0] == "" {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, false
		}
	}
	if start < 0 || start >= size || end >= size || start > end {
		return 0, 0, false
	}
	return int(start), int(end) + 1, true
}

func writeS3Error(w http.ResponseWriter, e *S3Error, r *http.Request) {
	status := e.Status
	if status == 0 {
		status = 400
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amz-request-id", "fake-request")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message><RequestId>fake-request</RequestId><HostId>fake</HostId></Error>`,
		xmlEscape(e.Code), xmlEscape(e.Message))
}

func writeXML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// extractXMLTags returns the inner text of every <tag>…</tag> occurrence.
func extractXMLTags(body, tag string) []string {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	var out []string
	rest := body
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			return out
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, closeTag)
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j+len(closeTag):]
	}
}
