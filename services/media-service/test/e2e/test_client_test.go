package e2e

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/grpc/pb"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	gorilla "github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// httpClient is shared by presigned URL operations.
var httpClient = &http.Client{Timeout: 90 * time.Second}

// unmarshalJSON decodes raw JSON into v.
func unmarshalJSON(raw []byte, v any) error { return json.Unmarshal(raw, v) }

// ---------------------------------------------------------------------------
// GraphQL
// ---------------------------------------------------------------------------

type gqlResp struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// gqlDo executes a GraphQL operation. Transport failures fail the test;
// GraphQL-level errors are returned so callers can assert on them.
func gqlDo(t *testing.T, token, query string, vars map[string]any) gqlResp {
	t.Helper()
	body := map[string]any{"query": query}
	if vars != nil {
		body["variables"] = vars
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal graphql request: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.GraphQLURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build graphql request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("graphql request: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read graphql response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("graphql http %d: %s", resp.StatusCode, truncate(raw, 400))
	}
	var out gqlResp
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode graphql response: %v (body %s)", err, truncate(raw, 400))
	}
	return out
}

// assertGraphQLError fails the test unless the response carries a GraphQL
// error whose message contains substr.
func assertGraphQLError(t *testing.T, res gqlResp, substr string) {
	t.Helper()
	if len(res.Errors) == 0 {
		t.Fatalf("expected graphql error containing %q, got none (data=%s)", substr, truncate(res.Data, 300))
	}
	for _, e := range res.Errors {
		if strings.Contains(e.Message, substr) {
			return
		}
	}
	t.Fatalf("expected graphql error containing %q, got %+v", substr, res.Errors)
}

// assertNoErrors fails the test on any GraphQL error.
func (r gqlResp) assertNoErrors(t *testing.T) {
	t.Helper()
	if len(r.Errors) > 0 {
		t.Fatalf("graphql errors: %+v", r.Errors)
	}
}

// op returns the operation envelope: {success, statusCode, message, data}.
func (r gqlResp) op(t *testing.T, name string) map[string]any {
	t.Helper()
	r.assertNoErrors(t)
	var data map[string]any
	if err := json.Unmarshal(r.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	env, ok := data[name].(map[string]any)
	if !ok {
		t.Fatalf("data.%s missing or not an object: %v", name, data)
	}
	return env
}

// opData returns the operation's inner data object and asserts success.
func (r gqlResp) opData(t *testing.T, name string) map[string]any {
	t.Helper()
	env := r.op(t, name)
	if ok, _ := env["success"].(bool); !ok {
		t.Fatalf("%s failed: statusCode=%v message=%v", name, env["statusCode"], env["message"])
	}
	inner, _ := env["data"].(map[string]any)
	return inner
}

// opDataSlice returns the operation's inner data when it is a list.
func (r gqlResp) opDataSlice(t *testing.T, name string) []any {
	t.Helper()
	env := r.op(t, name)
	if ok, _ := env["success"].(bool); !ok {
		t.Fatalf("%s failed: statusCode=%v message=%v", name, env["statusCode"], env["message"])
	}
	slice, _ := env["data"].([]any)
	return slice
}

// opMessage returns the operation message without requiring success.
func (r gqlResp) opMessage(t *testing.T, name string) string {
	t.Helper()
	env := r.op(t, name)
	msg, _ := env["message"].(string)
	return msg
}

// mintToken builds a valid HS256 JWT the service accepts.
func mintToken(t *testing.T, userID, role string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now()
	claims := map[string]any{"sub": userID, "role": role, "exp": now.Add(2 * time.Hour).Unix(), "iat": now.Unix()}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signing := header + "." + payload
	mac := hmac.New(sha256.New, []byte(cfg.JWTSecret))
	mac.Write([]byte(signing))
	return signing + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// uniqueUserID returns a fresh owner identity so quota, rate limits and DLQ
// assertions never leak between tests.
func uniqueUserID(t *testing.T) string {
	t.Helper()
	return "e2e-user-" + uuid.NewString()
}

// ---------------------------------------------------------------------------
// Upload flow
// ---------------------------------------------------------------------------

type UploadInput struct {
	FileName       string
	ContentType    string
	DeliveryID     string
	IdempotencyKey string
	Size           int64
	Checksum       string
}

type PresignedPart struct {
	PartNumber int    `json:"partNumber"`
	URL        string `json:"presignedUrl"`
}

type UploadSession struct {
	MediaID    string
	UploadID   string
	S3UploadID string
	PartSize   int64
	TotalParts int
	ExpiresAt  float64
	Presigned  []PresignedPart
	Token      string
}

type CompletedPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"eTag"`
}

const createUploadSessionMutation = `
mutation($input: CreateUploadSessionInput!) {
  createUploadSession(input: $input) {
    success statusCode message
    data { mediaId uploadId s3UploadId partSize totalParts expiresAt
           presignedParts { partNumber presignedUrl } }
  }
}`

// CreateUploadSession opens a multipart session through GraphQL.
func CreateUploadSession(t *testing.T, token string, in UploadInput) UploadSession {
	t.Helper()
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "e2e-" + uuid.NewString()
	}
	res := gqlDo(t, token, createUploadSessionMutation, map[string]any{"input": map[string]any{
		"fileName":       in.FileName,
		"contentType":    in.ContentType,
		"size":           in.Size,
		"checksum":       in.Checksum,
		"deliveryId":     in.DeliveryID,
		"idempotencyKey": in.IdempotencyKey,
	}})
	data := res.opData(t, "createUploadSession")
	session := UploadSession{
		MediaID:    str(data["mediaId"]),
		UploadID:   str(data["uploadId"]),
		S3UploadID: str(data["s3UploadId"]),
		PartSize:   int64(num(data["partSize"])),
		TotalParts: int(num(data["totalParts"])),
		ExpiresAt:  num(data["expiresAt"]),
		Token:      token,
	}
	rawParts, _ := data["presignedParts"].([]any)
	for _, rp := range rawParts {
		m, _ := rp.(map[string]any)
		session.Presigned = append(session.Presigned, PresignedPart{
			PartNumber: int(num(m["partNumber"])),
			URL:        str(m["presignedUrl"]),
		})
	}
	if session.UploadID == "" || len(session.Presigned) == 0 {
		t.Fatalf("unexpected createUploadSession payload: %v", data)
	}
	return session
}

// PutParts uploads payload directly to the presigned S3 URLs and returns the
// part ETags required by completeUpload.
func PutParts(t *testing.T, session UploadSession, payload []byte) []CompletedPart {
	t.Helper()
	if session.PartSize <= 0 {
		t.Fatalf("invalid partSize %d", session.PartSize)
	}
	wantParts := (int64(len(payload)) + session.PartSize - 1) / session.PartSize
	if int(wantParts) != len(session.Presigned) {
		t.Fatalf("payload of %d bytes needs %d parts but session has %d", len(payload), wantParts, len(session.Presigned))
	}
	client := &http.Client{Timeout: 60 * time.Second}
	parts := make([]CompletedPart, 0, len(session.Presigned))
	for i, p := range session.Presigned {
		start := int64(i) * session.PartSize
		end := start + session.PartSize
		if end > int64(len(payload)) {
			end = int64(len(payload))
		}
		etag := putPart(t, client, p.URL, payload[start:end])
		parts = append(parts, CompletedPart{PartNumber: p.PartNumber, ETag: etag})
	}
	return parts
}

func putPart(t *testing.T, client *http.Client, url string, body []byte) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build part request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload part: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		t.Fatalf("upload part http %d: %s", resp.StatusCode, truncate(raw, 400))
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("part upload returned no ETag (status %d)", resp.StatusCode)
	}
	return etag
}

const completeUploadMutation = `
mutation($input: CompleteUploadInput!) {
  completeUpload(input: $input) {
    success statusCode message
    data { mediaId status }
  }
}`

// CompleteUpload finalises the session and returns the operation envelope data.
func CompleteUpload(t *testing.T, session UploadSession, parts []CompletedPart) map[string]any {
	t.Helper()
	res := gqlDo(t, session.Token, completeUploadMutation, map[string]any{"input": map[string]any{
		"uploadId":       session.UploadID,
		"parts":          parts,
		"idempotencyKey": "complete-" + uuid.NewString(),
	}})
	return res.opData(t, "completeUpload")
}

const abortUploadMutation = `
mutation($uploadId: String!) {
  abortUpload(uploadId: $uploadId) { success statusCode message data }
}`

// AbortUpload cancels a session.
func AbortUpload(t *testing.T, token, uploadID string) gqlResp {
	t.Helper()
	return gqlDo(t, token, abortUploadMutation, map[string]any{"uploadId": uploadID})
}

// UploadBytes runs the whole happy path: session -> S3 parts -> complete.
// It returns the media id.
func UploadBytes(t *testing.T, token, fileName, contentType string, payload []byte) string {
	t.Helper()
	session := CreateUploadSession(t, token, UploadInput{
		FileName:    fileName,
		ContentType: contentType,
		Size:        int64(len(payload)),
	})
	parts := PutParts(t, session, payload)
	data := CompleteUpload(t, session, parts)
	mediaID := str(data["mediaId"])
	if mediaID == "" {
		t.Fatalf("completeUpload returned no mediaId: %v", data)
	}
	return mediaID
}

const getMediaQuery = `
query($mediaId: String!) {
  media(mediaId: $mediaId) {
    success statusCode message
    data { mediaId ownerId fileName contentType mediaType size objectKey status createdAt
           versions { versionType objectKey contentType size width height durationMs } }
  }
}`

// GetMedia returns the media envelope data, or nil when the media is missing.
func GetMedia(t *testing.T, token, mediaID string) map[string]any {
	t.Helper()
	res := gqlDo(t, token, getMediaQuery, map[string]any{"mediaId": mediaID})
	if len(res.Errors) > 0 {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal(res.Data, &root); err != nil {
		t.Fatalf("decode media query: %v", err)
	}
	env, _ := root["media"].(map[string]any)
	if env == nil {
		return nil
	}
	if ok, _ := env["success"].(bool); !ok {
		return nil
	}
	inner, _ := env["data"].(map[string]any)
	if inner == nil || str(inner["mediaId"]) == "" {
		return nil
	}
	return inner
}

const getDownloadURLQuery = `
query($mediaId: String!, $versionType: String, $expirySeconds: Int) {
  downloadUrl(mediaId: $mediaId, versionType: $versionType, expirySeconds: $expirySeconds) {
    success statusCode message
    data { url expiresAt contentType }
  }
}`

// DownloadURL returns the inner data of the downloadUrl query.
func DownloadURL(t *testing.T, token, mediaID, versionType string, expiry int) gqlResp {
	t.Helper()
	return gqlDo(t, token, getDownloadURLQuery, map[string]any{
		"mediaId": mediaID, "versionType": versionType, "expirySeconds": expiry,
	})
}

const getUploadStatusQuery = `
query($uploadId: String!) {
  uploadStatus(uploadId: $uploadId) {
    success statusCode message
    data { uploadId status totalParts completedParts missingParts expiresAt }
  }
}`

// UploadStatus returns the inner data of the uploadStatus query.
func UploadStatus(t *testing.T, token, uploadID string) gqlResp {
	t.Helper()
	return gqlDo(t, token, getUploadStatusQuery, map[string]any{"uploadId": uploadID})
}

const getQuotaQuery = `
query {
  quota {
    success statusCode message
    data { usedBytes quotaBytes remainingBytes activeUploads maxConcurrentUploads }
  }
}`

// Quota returns the inner data of the quota query.
func Quota(t *testing.T, token string) map[string]any {
	t.Helper()
	return gqlDo(t, token, getQuotaQuery, nil).opData(t, "quota")
}

const deleteMediaMutation = `
mutation($mediaId: String!, $idempotencyKey: String) {
  deleteMedia(mediaId: $mediaId, idempotencyKey: $idempotencyKey) {
    success statusCode message data
  }
}`

// DeleteMedia issues the delete mutation.
func DeleteMedia(t *testing.T, token, mediaID string) gqlResp {
	t.Helper()
	return gqlDo(t, token, deleteMediaMutation, map[string]any{
		"mediaId": mediaID, "idempotencyKey": "del-" + uuid.NewString(),
	})
}

// WaitForMediaStatus polls getMedia until the media reaches want (or timeout).
func WaitForMediaStatus(t *testing.T, token, mediaID, want string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		if m := GetMedia(t, token, mediaID); m != nil {
			last = str(m["status"])
			if last == want {
				return m
			}
		}
		time.Sleep(1500 * time.Millisecond)
	}
	t.Fatalf("media %s never reached status %s (last=%s)", mediaID, want, last)
	return nil
}

// WaitForVersion polls until a version of the given type exists.
func WaitForVersion(t *testing.T, token, mediaID, versionType string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m := GetMedia(t, token, mediaID); m != nil {
			for _, raw := range m["versions"].([]any) {
				v, _ := raw.(map[string]any)
				if str(v["versionType"]) == versionType {
					return v
				}
			}
		}
		time.Sleep(1500 * time.Millisecond)
	}
	t.Fatalf("media %s never produced version %q", mediaID, versionType)
	return nil
}

// ---------------------------------------------------------------------------
// AWS clients (LocalStack)
// ---------------------------------------------------------------------------

func awsConfig(t *testing.T) aws.Config {
	t.Helper()
	res, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AWSKey, cfg.AWSSecret, "")),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}
	return res
}

func s3Client(t *testing.T) *s3.Client {
	t.Helper()
	return s3.NewFromConfig(awsConfig(t), func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		o.UsePathStyle = true
	})
}

func ddbClient(t *testing.T) *dynamodb.Client {
	t.Helper()
	return dynamodb.NewFromConfig(awsConfig(t), func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(cfg.DDBEndpoint)
	})
}

// ensureInfra creates the bucket and table if they do not exist yet.
func ensureInfra() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// The SDK clients need testing.T; build them directly here instead.
	res, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AWSKey, cfg.AWSSecret, "")),
	)
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}
	s3c := s3.NewFromConfig(res, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		o.UsePathStyle = true
	})
	if _, err := s3c.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(cfg.Bucket)}); err != nil {
		if !strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") && !strings.Contains(err.Error(), "BucketAlreadyExists") {
			return fmt.Errorf("create bucket: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Kafka (advertised listeners point at the in-cluster name, so every dial is
// rewritten to the configured broker address).
// ---------------------------------------------------------------------------

func kafkaClient(t *testing.T) *kafkago.Client {
	t.Helper()
	tr := &kafkago.Transport{
		ClientID: "media-service-e2e",
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, cfg.KafkaBroker)
		},
	}
	return &kafkago.Client{Addr: kafkago.TCP(cfg.KafkaBroker), Transport: tr}
}

// ProduceKafka publishes a raw record on the given topic.
func ProduceKafka(t *testing.T, topic string, key, value []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := kafkaClient(t)
	resp, err := client.Produce(ctx, &kafkago.ProduceRequest{
		Topic:     topic,
		Partition: 0,
		Records: kafkago.NewRecordReader(kafkago.Record{
			Key:   kafkago.NewBytes(key),
			Value: kafkago.NewBytes(value),
			Time:  time.Now(),
		}),
	})
	if err != nil {
		t.Fatalf("produce to %s: %v", topic, err)
	}
	if resp != nil && resp.Error != nil {
		t.Fatalf("produce to %s: %v", topic, resp.Error)
	}
}

// FetchKafka reads up to max records from the start of a topic.
func FetchKafka(t *testing.T, topic string, maxWait time.Duration, max int) []kafkago.Record {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), maxWait+10*time.Second)
	defer cancel()
	client := kafkaClient(t)
	resp, err := client.Fetch(ctx, &kafkago.FetchRequest{
		Topic:     topic,
		Partition: 0,
		Offset:    0,
		MinBytes:  1,
		MaxBytes:  10 << 20,
		MaxWait:   maxWait,
	})
	if err != nil {
		return nil
	}
	if resp.Error != nil {
		return nil
	}
	var out []kafkago.Record
	reader := resp.Records
	if reader == nil {
		return nil
	}
	for {
		rec, err := reader.ReadRecord()
		if err != nil {
			break
		}
		out = append(out, *rec)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

// RecordValue reads the payload of a fetched record.
func RecordValue(t *testing.T, r kafkago.Record) []byte {
	t.Helper()
	if r.Value == nil {
		return nil
	}
	data, err := kafkago.ReadAll(r.Value)
	if err != nil {
		t.Fatalf("read record value: %v", err)
	}
	return data
}

// RecordKey reads the key of a fetched record.
func RecordKey(t *testing.T, r kafkago.Record) []byte {
	t.Helper()
	if r.Key == nil {
		return nil
	}
	data, err := kafkago.ReadAll(r.Key)
	if err != nil {
		t.Fatalf("read record key: %v", err)
	}
	return data
}

// ---------------------------------------------------------------------------
// WebSocket
// ---------------------------------------------------------------------------

// DialWS opens an authenticated websocket connection to the hub.
func DialWS(t *testing.T, token string) *gorilla.Conn {
	t.Helper()
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	conn, _, err := gorilla.DefaultDialer.Dial(cfg.WSURL, header)
	if err != nil {
		t.Fatalf("dial websocket %s: %v", cfg.WSURL, err)
	}
	t.Cleanup(func() { conn.Close() })
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	return conn
}

// WSReadJSON reads one JSON message from the socket.
func WSReadJSON(t *testing.T, conn *gorilla.Conn, timeout time.Duration) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode websocket message %s: %v", truncate(data, 200), err)
	}
	return out
}

// WaitForWSEvent reads messages until one with the given type arrives.
func WaitForWSEvent(t *testing.T, conn *gorilla.Conn, eventType string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(remaining))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read websocket message while waiting for %s: %v", eventType, err)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if str(msg["type"]) == eventType {
			return msg
		}
	}
	t.Fatalf("websocket event %q not received within %s", eventType, timeout)
	return nil
}

// ---------------------------------------------------------------------------
// gRPC
// ---------------------------------------------------------------------------

// DialGRPC opens an insecure client connection to the media gRPC server.
func DialGRPC(t *testing.T) pb.MediaServiceClient {
	t.Helper()
	conn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial grpc %s: %v", cfg.GRPCAddr, err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewMediaServiceClient(conn)
}

// ---------------------------------------------------------------------------
// Payload fixtures
// ---------------------------------------------------------------------------

// PNGBytes renders a real PNG image of the requested size.
func PNGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 3), G: uint8(y * 3), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// JPEGBytes renders a real JPEG image of the requested size.
func JPEGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(255 - x), G: uint8(y), B: 64, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// EXEBytes returns a Windows-PE-looking payload used for extension spoofing.
func EXEBytes() []byte {
	buf := make([]byte, 1024)
	copy(buf, []byte("MZ"))
	copy(buf[0x3c:], []byte{0x40, 0x00, 0x00, 0x00})
	copy(buf[0x40:], []byte("PE\x00\x00"))
	for i := 0x44; i < len(buf); i++ {
		buf[i] = byte(i % 251)
	}
	return buf
}

// TinyMP4 loads the committed 1-second H.264 clip used by the video scenario.
func TinyMP4(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "tiny.mp4"))
	if err != nil {
		t.Fatalf("read testdata/tiny.mp4: %v", err)
	}
	return data
}

// ChecksumSHA256 returns the hex digest used by the optional checksum field.
func ChecksumSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// NATS
// ---------------------------------------------------------------------------

// SubscribeNATS subscribes to a subject and returns a channel of raw payloads.
func SubscribeNATS(t *testing.T, subject string) <-chan []byte {
	t.Helper()
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		t.Fatalf("connect nats %s: %v", cfg.NATSURL, err)
	}
	t.Cleanup(func() { nc.Close() })
	ch := make(chan []byte, 64)
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		select {
		case ch <- append([]byte(nil), m.Data...):
		default:
		}
	})
	if err != nil {
		t.Fatalf("subscribe %s: %v", subject, err)
	}
	t.Cleanup(func() { sub.Unsubscribe() })
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush nats: %v", err)
	}
	return ch
}

// WaitForNATSMention reads from ch until a payload mentions needle or timeout.
// The realtime publisher double-marshals payloads (JSON -> base64 string on the
// wire — a known production quirk), so decoded payloads are searched too.
func WaitForNATSMention(t *testing.T, ch <-chan []byte, needle string, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case data := <-ch:
			if strings.Contains(string(data), needle) {
				return data
			}
			if decoded := decodeDoubleMarshaled(data); len(decoded) > 0 && strings.Contains(string(decoded), needle) {
				return data
			}
		case <-deadline:
			t.Fatalf("no nats message mentioning %q within %s", needle, timeout)
			return nil
		}
	}
}

// decodeDoubleMarshaled unwraps a JSON string of base64 (the double-marshal
// wire format) back into the original JSON payload.
func decodeDoubleMarshaled(data []byte) []byte {
	var s string
	if err := json.Unmarshal(data, &s); err != nil || s == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return []byte(s)
	}
	return raw
}

// FetchBytes downloads a URL (used for presigned GET verification).
func FetchBytes(t *testing.T, url string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build get request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", truncate([]byte(url), 120), err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode/100 != 2 {
		t.Fatalf("get http %d: %s", resp.StatusCode, truncate(raw, 200))
	}
	return raw
}

// EICARString is the standard antivirus test signature.
const EICARString = "X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"

// ProbeTCP reports whether addr accepts a TCP connection.
func ProbeTCP(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// sha1Hex is only used for deterministic kafka keys in tests.
func sha1Hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
