# Search Service - التوثيق الشامل

## المحتويات

1. [نظرة عامة](#نظرة-عامة)
2. [بنية المشروع](#بنية-المشروع)
3. [العلاقات مع الخدمات الأخرى](#العلاقات-مع-الخدمات-الأخرى)
4. [الباكد المشتركة (Shared Packages)](#الباكد-المشتركة)
5. [OpenSearch - كل ما تحتاج معرفته](#opensearch)
6. [Index Operations and Management](#index-operations-and-management)
7. [البحث والاستعلام](#البحث-واستعلام)
8. [Kafka Event Consumers](#kafka-event-consumers)
9. [Redis Cache](#redis-cache)
10. [التكوين (Configuration)](#التكوين)
11. [مشكلة GraphQL](#graphql)

---

## نظرة عامة

`search-service` هي سيرفيس مسؤولة عن **فهرسة البيانات والبحث** في نظام التوصيل في الوقت الحقيقي. السيرفيس دي بتستخدم **OpenSearch** كمحرك بحث رئيسي، و**Redis** للتخزين المؤقت (Cache)، و**Kafka** للاستماع للأحداث اللي بتيجي من السيرفيس التانية.

### الوظائف الرئيسية:

- **فهرسة التوصيلات (Deliveries)**: إضافة وتحديث وحذف وثائق التوصيلات في OpenSearch
- **فهرسة السائقين (Drivers)**: إضافة وتحديث وحذف بيانات السائقين
- **فهرسة الوسائط (Media)**: إضافة وتحديث وحذف بيانات الملفات الوسائطية
- **بحث متقدم**: بحث نصي كامل، بحث جغرافي (Geo Search)، وبحث م autocomplete
- **إعادة الفهرسة (Reindex)**: إنشاء نسخة جديدة من الفهرس مع التبديل السلس

---

## بنية المشروع

```
search-service/
├── cmd/server/main.go                    # نقطة الدخول الرئيسية
├── internal/
│   ├── application/
│   │   ├── indexing/service.go            # خدمة الفهرسة (إضافة/تحديث/حذف)
│   │   ├── search/service.go             # خدمة البحث مع Cache
│   │   └── reindex/service.go            # خدمة إعادة الفهرسة
│   ├── domain/search/
│   │   ├── result.go                     # النماذج (Documents, Queries, Results)
│   │   ├── repository.go                 # واجهة المستودع (Interface)
│   │   └── errors.go                     # أخطاء مخصصة
│   ├── infrastructure/
│   │   ├── opensearch/
│   │   │   ├── client.go                 # عميل OpenSearch
│   │   │   ├── index_manager.go          # إدارة الفهارس (إنشاء/maps)
│   │   │   └── repository.go             # تطبيق واجهة المستودع على OpenSearch
│   │   ├── kafka/consumer.go             # مستقبل أحداث Kafka
│   │   └── redis/cache.go                # عميل Redis للتخزين المؤقت
│   ├── interfaces/
│   │   ├── graphql/server.go             # خادم GraphQL
│   │   └── health/handler.go             # معالجات الصحة (Health Checks)
│   └── config/config.go                  # إعدادات التطبيق
├── Dockerfile
└── go.mod
```

---

## العلاقات مع الخدمات الأخرى

### الخدمات في المشروع:

```
┌─────────────────────────────────────────────────────────────┐
│                    Realtime Delivery System                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │user-service │    │media-service│    │search-service│     │
│  │             │    │             │    │             │      │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘     │
│         │                  │                  │             │
│         │    ┌─────────────┼─────────────┐    │             │
│         │    │             │             │    │             │
│         ▼    ▼             ▼             ▼    ▼             │
│  ┌─────────────────────────────────────────────────┐       │
│  │                   Kafka                         │       │
│  └─────────────────────────────────────────────────┘       │
│         │                  │                  │             │
│         ▼                  ▼                  ▼             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │notification │    │ realtime-   │    │ api-gateway │     │
│  │  service    │    │  service    │    │             │     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### العلاقة مع كل سيرفيس:

#### 1. **media-service** ← search-service

- **media-service** بتنشر أحداث (Events) في Kafka لما الملفات الوسائطية تتجهز أو تتوقف
- **search-service** بتستقبل الأحداث دي وتقيم الملفات في فهرس `media` في OpenSearch
- **الأحداث المستلمة**:
  - `media.ready` - لما الملف يبقى جاهز بعد المعالجة
  - `media.deleted` - لما الملف يت”

#### 2. **user-service** ← search-service

- **user-service** بتنشر أحداث في Kafka لما بيانات المستخدم تتغير
- **search-service** بتستقبل الأحداث وتقوم بتحديث فهرس المستخدمين لو موجود

#### 3. **delivery-service** (لو موجود) ← search-service

- **delivery-service** بتنشر أحداث في://{delivery.created, delivery.completed, إلخ}
- **search-service** بتستقبل الأحداث وتقيم التوصيلات في فهرس `deliveries`

#### 4. **driver-service** (لو موجود) ← search-service

- **driver-service** بتنشر أحداث في://{driver.created, driver.updated, إلخ}
- **search-service** بتستقبل الأحداث وتقيم السائقين في فهرس `drivers`

#### 5. **api-gateway** ← search-service

- **api-gateway** بيبعت طلبات البحث إلى search-service عبر GraphQL
- search-service بترد بنتائج البحث

#### 6. **realtime-service** ← search-service

- **realtime-service** ممكن يستخدم search-service للبحث عن التوصيلات القريبة في الوقت الحقيقي

---

## الباكد المشتركة

المشروع بيستخدم باكد مشتركة موجودة في `packages/go/`:

### 1. **packages/go/events** - أحداث مشتركة

الباكد دي بتوفر تعريفات الأحداث اللي بتتمر بين جميع الخدمات:

```go
// أنواع الأحداث
DeliveryCreated        = "delivery.created"
DeliveryDriverAssigned = "delivery.driver.assigned"
DeliveryDriverAccepted = "delivery.driver.accepted"
DeliveryPickedUp       = "delivery.picked_up"
DeliveryInTransit      = "delivery.in_transit"
DeliveryCompleted      = "delivery.completed"
DeliveryCancelled      = "delivery.cancelled"
DeliveryDeleted        = "delivery.deleted"

DriverCreated = "driver.created"
DriverUpdated = "driver.updated"
DriverDeleted = "driver.deleted"

MediaReady   = "media.ready"
MediaDeleted = "media.deleted"
```

**الهيكل العام للحدث (EventEnvelope):**

```json
{
  "eventId": "unique-event-id",
  "eventType": "delivery.created",
  "traceId": "trace-id-for-distributed-tracing",
  "timestamp": 1692000000000,
  "payload": { ... }
}
```

### 2. **packages/go/kafka** - عميل Kafka مشترك

الباكد دي بتوفر:
- `Consumer` - مستقبل أحداث Kafka مع دعم إعادة المحاولة (Retry) وقائمة الانتظار الميتة (DLQ)
- `Producer` - منتج أحداث Kafka

**特点:**
- دعم `MaxRetries` مع `DLQ` (Dead Letter Queue)
- كل مستقبل ليه `Consumer Group` منفصل
- دعم `SASL` للمصادقة

### 3. **packages/go/logging** - تسجيل مشترك

بتوفر تسجيل موحد لجميع الخدمات باستخدام `slog`.

### 4. **packages/go/metrics** - مقاييس مشتركة

بتوفر مقاييس Prometheus لجميع الخدمات.

---

## OpenSearch

### ما هو OpenSearch؟

OpenSearch هو محرك بحث مفتوح المصدر (fork من Elasticsearch) بيستخدم لـ:
- **البحث النصي الكامل (Full-Text Search)**
- **البحث الجغرافي (Geo Search)**
- **البحث الم autocomplete**
- **تحليل النصوص (Text Analysis)**
- **التخزين والفهرسة (Indexing)**

### التكوين:

```yaml
OPENSEARCH_URL: http://opensearch-srv:9200
OPENSEARCH_USERNAME: admin
OPENSEARCH_PASSWORD: admin
OPENSEARCH_INSECURE: true  # للاختبار فقط
```

### الفهارس (Indices):

السيرفис بتستخدم 3 فهارس رئيسية:

#### 1. **deliveries** - فهرس التوصيلات

```json
{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "autocomplete_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "edge_ngram_filter"]
        }
      },
      "filter": {
        "edge_ngram_filter": {
          "type": "edge_ngram",
          "min_gram": 2,
          "max_gram": 20
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "delivery_id": { "type": "keyword" },
      "customer_id": { "type": "keyword" },
      "driver_id": { "type": "keyword" },
      "status": { "type": "keyword" },
      "pickup": {
        "properties": {
          "city": {
            "type": "text",
            "analyzer": "autocomplete_analyzer",
            "search_analyzer": "standard",
            "fields": { "keyword": { "type": "keyword" } }
          },
          "country": { "type": "keyword" },
          "location": { "type": "geo_point" }
        }
      },
      "dropoff": {
        "properties": {
          "city": {
            "type": "text",
            "analyzer": "autocomplete_analyzer",
            "search_analyzer": "standard",
            "fields": { "keyword": { "type": "keyword" } }
          },
          "country": { "type": "keyword" },
          "location": { "type": "geo_point" }
        }
      },
      "created_at": { "type": "date" },
      "updated_at": { "type": "date" },
      "indexed_at": { "type": "date" },
      "source_version": { "type": "long" }
    }
  }
}
```

#### 2. **drivers** - فهرس السائقين

```json
{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "autocomplete_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "edge_ngram_filter"]
        }
      },
      "filter": {
        "edge_ngram_filter": {
          "type": "edge_ngram",
          "min_gram": 2,
          "max_gram": 20
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "driver_id": { "type": "keyword" },
      "name": {
        "type": "text",
        "analyzer": "autocomplete_analyzer",
        "search_analyzer": "standard",
        "fields": { "keyword": { "type": "keyword" } }
      },
      "status": { "type": "keyword" },
      "vehicle_type": { "type": "keyword" },
      "rating": { "type": "float" },
      "location": { "type": "geo_point" },
      "updated_at": { "type": "date" },
      "indexed_at": { "type": "date" },
      "source_version": { "type": "long" }
    }
  }
}
```

#### 3. **media** - فهرس الوسائط

```json
{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "autocomplete_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "edge_ngram_filter"]
        }
      },
      "filter": {
        "edge_ngram_filter": {
          "type": "edge_ngram",
          "min_gram": 2,
          "max_gram": 20
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "media_id": { "type": "keyword" },
      "owner_id": { "type": "keyword" },
      "file_name": {
        "type": "text",
        "analyzer": "autocomplete_analyzer",
        "search_analyzer": "standard",
        "fields": { "keyword": { "type": "keyword" } }
      },
      "mime_type": { "type": "keyword" },
      "media_type": { "type": "keyword" },
      "status": { "type": "keyword" },
      "size": { "type": "long" },
      "created_at": { "type": "date" },
      "indexed_at": { "type": "date" },
      "source_version": { "type": "long" }
    }
  }
}
```

### Analysers المستخدمة:

#### **autocomplete_analyzer**
- **Tokenizer**: `standard` - بيقسم النص لكلمات
- **Filters**:
  - `lowercase` - بيحول كل حرف لحروف صغيرة
  - `edge_ngram_filter` - بيولد أحرف ناقصة من البداية (مثلاً: "ca" → "ca", "car")

#### **edge_ngram_filter**
- `min_gram: 2` - أقل عدد أحرف هو 2
- `max_gram: 20` - أكبر عدد أحرف هو 20
- **الاستخدام**: بيستخدم لعمل autocomplete (مقترحات أثناء الكتابة)

### أنواع البيانات (Data Types):

| Type | الاستخدام |
|------|-----------|
| `keyword` | للقيم الثابتة (status, id, country) |
| `text` | للنصوص القابلة للبحث |
| `geo_point` | لإحداثيات GPS (lat, lon) |
| `date` | للتاريخ والوقت |
| `float` | للأرقام العشرية (rating) |
| `long` | للأرقام الصحيحة الكبيرة (version) |

---

## Index Operations and Management

### 1. **إنشاء الفهارس (Index Creation)**

عند تشغيل السيرفيس، بيتم إنشاء الفهارس تلقائياً:

```go
// internal/infrastructure/opensearch/index_manager.go
func (im *IndexManager) EnsureIndices(ctx context.Context) error {
    indices := []struct {
        Alias   string
        Version string
        Mapping string
    }{
        {
            Alias:   "deliveries",
            Version: "deliveries-v1",
            Mapping: deliveriesMapping,
        },
        {
            Alias:   "drivers",
            Version: "drivers-v1",
            Mapping: driversMapping,
        },
        {
            Alias:   "media",
            Version: "media-v1",
            Mapping: mediaMapping,
        },
    }

    for _, idx := range indices {
        if err := im.createIndexIfNotExists(ctx, idx.Version, idx.Alias, idx.Mapping); err != nil {
            return fmt.Errorf("failed to ensure index %s: %w", idx.Alias, err)
        }
    }
    return nil
}
```

**العملية:**
1. بيتحقق إذا الفهرس موجود (Status 200)
2. لو مش موجود، بينشئ فهرس جديد بالـ Mapping
3. بيعمل Alias للفهرس (مثلاً: `deliveries` → `deliveries-v1`)

### 2. **إضافة/تحديث الوثائق (Upsert)**

```go
// internal/infrastructure/opensearch/repository.go
func (r *Repository) upsertDoc(ctx context.Context, index, id string, doc interface{}, incomingVersion int64) error {
    // فحص الإصدار لتجنب الكتابة فوق بيانات أحدث
    currentVersion, err := r.GetDocumentVersion(ctx, index, id)
    if err == nil && currentVersion >= incomingVersion && currentVersion != -1 {
        slog.Warn("Skipping stale document indexing",
            "index", index, "id", id,
            "currentVersion", currentVersion,
            "incomingVersion", incomingVersion)
        return nil
    }

    data, err := json.Marshal(doc)
    if err != nil {
        return err
    }

    req := opensearchapi.IndexReq{
        Index:      index,
        DocumentID: id,
        Body:       bytes.NewReader(data),
    }

    res, err := r.client.Do(ctx, req, nil)
    if err != nil {
        return fmt.Errorf("%w: %v", search.ErrSearchUnavailable, err)
    }
    defer res.Body.Close()

    if res.StatusCode >= 400 {
        respBody, _ := io.ReadAll(res.Body)
        return fmt.Errorf("indexing error (%d): %s", res.StatusCode, string(respBody))
    }
    return nil
}
```

**المميزات:**
- **Version Check**: بيتحقق من إصدار الوثيقة الحالية قبل الكتابة لتجنب التضارب
- **Idempotent**: لو الوثيقة موجودة بنفس الإصدار أو أحدث، مش بيكتبها تاني
- **Auto Timestamp**: بيحط `indexed_at` تلقائياً

### 3. **حذف الوثائق (Delete)**

```go
func (r *Repository) deleteDoc(ctx context.Context, index, id string) error {
    req := opensearchapi.DocumentDeleteReq{
        Index:      index,
        DocumentID: id,
    }

    res, err := r.client.Do(ctx, req, nil)
    if err != nil {
        return fmt.Errorf("%w: %v", search.ErrSearchUnavailable, err)
    }
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
        return fmt.Errorf("delete failed with status %d", res.StatusCode)
    }
    return nil
}
```

**ملاحظة**: بيرجع OK حتى لو الوثيقة مش موجودة (NotFound)

### 4. **الإضافة بالجملة (Bulk Upsert)**

```go
func (r *Repository) BulkUpsertDeliveries(ctx context.Context, docs []search.DeliveryDocument) error {
    var buf bytes.Buffer
    for _, doc := range docs {
        doc.IndexedAt = time.Now().UTC()
        meta := fmt.Sprintf(`{"index":{"_index":"deliveries","_id":"%s"}}`+"\n", doc.DeliveryID)
        docBytes, err := json.Marshal(doc)
        if err != nil {
            continue
        }
        buf.WriteString(meta)
        buf.Write(docBytes)
        buf.WriteString("\n")
    }
    return r.executeBulk(ctx, &buf)
}
```

**التنسيق:**
```
{"index":{"_index":"deliveries","_id":"doc1"}}
{"field1":"value1","field2":"value2"}
{"index":{"_index":"deliveries","_id":"doc2"}}
{"field1":"value3","field2":"value4"}
```

### 5. **البحث (Search Operations)**

#### **بحث عام مع Fuzziness**
```go
func buildDeliveryQuery(q search.DeliverySearchQuery) map[string]interface{} {
    must := []map[string]interface{}{}
    filter := []map[string]interface{}{}

    if strings.TrimSpace(q.Query) != "" {
        must = append(must, map[string]interface{}{
            "multi_match": map[string]interface{}{
                "query":     q.Query,
                "fields":    []string{"delivery_id^3", "pickup.city^2", "dropoff.city^2", "status"},
                "fuzziness": "AUTO",  // بيدعم الأخطاء الإملائية
            },
        })
    }
    // ... باقي الفلاتر
}
```

**المField Weighting:**
- `delivery_id^3` - أولوية عالية جداً
- `pickup.city^2` - أولوية عالية
- `status` - أولوية عادية

#### **البحث الجغرافي (Geo Search)**
```go
func (r *Repository) NearbyDeliveries(ctx context.Context, q search.GeoSearchQuery) (search.SearchResult[search.DeliveryDocument], error) {
    queryDsl := map[string]interface{}{
        "query": map[string]interface{}{
            "bool": map[string]interface{}{
                "filter": []map[string]interface{}{
                    {
                        "geo_distance": map[string]interface{}{
                            "distance": fmt.Sprintf("%.2fkm", q.RadiusKm),
                            "pickup.location": map[string]interface{}{
                                "lat": q.Lat,
                                "lon": q.Lon,
                            },
                        },
                    },
                },
            },
        },
        "sort": []map[string]interface{}{
            {
                "_geo_distance": map[string]interface{}{
                    "pickup.location": map[string]interface{}{
                        "lat": q.Lat,
                        "lon": q.Lon,
                    },
                    "order":         "asc",    // الأقرب أولاً
                    "unit":          "km",
                    "mode":          "min",
                    "distance_type": "arc",
                },
            },
        },
    }
    return executeSearch[search.DeliveryDocument](ctx, r.client, "deliveries", queryDsl, q.Pagination)
}
```

#### **البحث الم autocomplete**
```go
func (r *Repository) Autocomplete(ctx context.Context, q search.AutocompleteQuery) (search.AutocompleteResult, error) {
    queryDsl := map[string]interface{}{
        "size": limit,
        "query": map[string]interface{}{
            "multi_match": map[string]interface{}{
                "query": q.Prefix,
                "type":  "bool_prefix",
                "fields": []string{
                    "pickup.city", "pickup.city._2gram", "pickup.city._3gram",
                    "dropoff.city", "dropoff.city._2gram", "dropoff.city._3gram",
                    "name", "name._2gram", "name._3gram",
                    "file_name", "file_name._2gram", "file_name._3gram",
                },
            },
        },
    }
    // ... باقي الكود
}
```

**الاستخدام**: بيستخدم الـ edge_ngram عشان يقترح كلمات أثناء الكتابة

### 6. **إعادة الفهرسة (Reindex)**

```go
func (r *Repository) Reindex(ctx context.Context, source, target string) error {
    reindexBody := map[string]interface{}{
        "source": map[string]interface{}{"index": source},
        "dest":   map[string]interface{}{"index": target},
    }
    b, err := json.Marshal(reindexBody)
    if err != nil {
        return err
    }

    req := opensearchapi.ReindexReq{
        Body: bytes.NewReader(b),
    }
    res, err := r.client.Do(ctx, req, nil)
    if err != nil {
        return err
    }
    defer res.Body.Close()

    if res.StatusCode >= 400 {
        resp, _ := io.ReadAll(res.Body)
        return fmt.Errorf("reindex failed: %s", string(resp))
    }
    return nil
}
```

### 7. **تبديل الـ Alias (Switch Alias)**

```go
func (r *Repository) SwitchAlias(ctx context.Context, alias, oldIndex, newIndex string) error {
    actions := []map[string]interface{}{
        {
            "add": map[string]interface{}{
                "index": newIndex,
                "alias": alias,
            },
        },
    }
    if oldIndex != "" {
        actions = append([]map[string]interface{}{
            {
                "remove": map[string]interface{}{
                    "index": oldIndex,
                    "alias": alias,
                },
            },
        }, actions...)
    }

    body := map[string]interface{}{
        "actions": actions,
    }
    b, _ := json.Marshal(body)

    req := opensearchapi.AliasesReq{
        Body: bytes.NewReader(b),
    }
    res, err := r.client.Do(ctx, req, nil)
    if err != nil {
        return err
    }
    defer res.Body.Close()
    return nil
}
```

**الاستخدام**: بيستخدم لتغيير الـ Alias من فهرس قديم لجديد بدون downtime

### 8. **التحقق من الصحة (Health Checks)**

```go
func (r *Repository) Ping(ctx context.Context) error {
    req := opensearchapi.PingReq{}
    res, err := r.client.Do(ctx, req, nil)
    if err != nil {
        return err
    }
    defer res.Body.Close()
    if res.StatusCode != http.StatusOK {
        return fmt.Errorf("ping returned %d", res.StatusCode)
    }
    return nil
}

func (r *Repository) ClusterHealth(ctx context.Context) (string, error) {
    req := opensearchapi.ClusterHealthReq{}
    res, err := r.client.Do(ctx, req, nil)
    if err != nil {
        return "red", err
    }
    defer res.Body.Close()

    var health struct {
        Status string `json:"status"`
    }
    if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
        return "unknown", err
    }
    return health.Status, nil
}
```

**الحالات:**
- `green` - كل الـ shards شغالة
- `yellow` - الـ primary shards شغالة بس الـ replicas مش كلها
- `red` - في shards مش شغالة

---

## البحث والاستعلام

### نماذج البيانات (Domain Models)

#### **DeliveryDocument**
```go
type DeliveryDocument struct {
    DeliveryID    string     `json:"delivery_id"`
    CustomerID    string     `json:"customer_id"`
    DriverID      string     `json:"driver_id,omitempty"`
    Status        string     `json:"status"`
    Pickup        GeoAddress `json:"pickup"`
    Dropoff       GeoAddress `json:"dropoff"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
    IndexedAt     time.Time  `json:"indexed_at"`
    SourceVersion int64      `json:"source_version"`
}
```

#### **DriverDocument**
```go
type DriverDocument struct {
    DriverID      string    `json:"driver_id"`
    Name          string    `json:"name"`
    Status        string    `json:"status"`       // AVAILABLE | BUSY | OFFLINE
    VehicleType   string    `json:"vehicle_type"` // CAR | MOTORCYCLE | TRUCK | BICYCLE
    Location      *GeoPoint `json:"location,omitempty"`
    Rating        float64   `json:"rating"`
    UpdatedAt     time.Time `json:"updated_at"`
    IndexedAt     time.Time `json:"indexed_at"`
    SourceVersion int64     `json:"source_version"`
}
```

#### **MediaDocument**
```go
type MediaDocument struct {
    MediaID       string    `json:"media_id"`
    OwnerID       string    `json:"owner_id"`
    FileName      string    `json:"file_name"`
    MimeType      string    `json:"mime_type"`
    MediaType     string    `json:"media_type"` // IMAGE | VIDEO | DOCUMENT
    Status        string    `json:"status"`     // READY
    Size          int64     `json:"size"`
    CreatedAt     time.Time `json:"created_at"`
    IndexedAt     time.Time `json:"indexed_at"`
    SourceVersion int64     `json:"source_version"`
}
```

### استعلامات البحث (Search Queries)

#### **DeliverySearchQuery**
```go
type DeliverySearchQuery struct {
    Query      string             // نص البحث
    Status     string             // فلتر الحالة
    City       string             // فلتر المدينة
    DriverID   string             // فلتر السائق
    CustomerID string             // فلتر العميل
    FromDate   *time.Time         // من تاريخ
    ToDate     *time.Time         // إلى تاريخ
    Geo        *GeoDistanceFilter // فلتر جغرافي
    Sort       []DeliverySearchSort // الترتيب
    Pagination PaginationInput    // Pagination

    UserID   string // للصلاحيات
    UserRole string // للصلاحيات
}
```

#### **GeoDistanceFilter**
```go
type GeoDistanceFilter struct {
    Lat      float64 // خط العرض
    Lon      float64 // خط الطول
    RadiusKm float64 // نصف القطر بالكيلومتر
}
```

### Pagination Cursor-Based

السيرفيس بتستخدم **Cursor-Based Pagination** عشان تتجنب مشاكل الـ Offset:

```go
type searchCursorPayload struct {
    SortValues []interface{} `json:"sv"`
}

func EncodeCursor(sortValues []interface{}) (string, error) {
    cp := searchCursorPayload{SortValues: sortValues}
    data, err := json.Marshal(cp)
    if err != nil {
        return "", fmt.Errorf("encode cursor: %w", err)
    }
    return base64.StdEncoding.EncodeToString(data), nil
}
```

**الاستخدام**:
- العميل بيبعت `cursor` من الطلب السابق
- السيرفيس بتحوله لـ `search_after` في OpenSearch
- ده بيخلي الـ pagination أسرع وأدق

---

## Kafka Event Consumers

السيرفيس بتستقبل أحداث من 13 topic في Kafka:

### **أحداث التوصيلات:**

| Topic | EventType | الوظيفة |
|-------|-----------|---------|
| `delivery.created` | `delivery.created` | إضافة توصيلة جديدة |
| `delivery.driver.assigned` | `delivery.driver.assigned` | تحديث تعيين السائق |
| `delivery.driver.accepted` | `delivery.driver.accepted` | تحديث قبول السائق |
| `delivery.picked_up` | `delivery.picked_up` | تحديث الاستلام |
| `delivery.in_transit` | `delivery.in_transit` | تحديث في الطريق |
| `delivery.completed` | `delivery.completed` | تحديث الاكتمال |
| `delivery.cancelled` | `delivery.cancelled` | تحديث الإلغاء |
| `delivery.deleted` | `delivery.deleted` | حذف التوصيلة |

### **أحداث السائقين:**

| Topic | EventType | الوظيفة |
|-------|-----------|---------|
| `driver.created` | `driver.created` | إضافة سائق جديد |
| `driver.updated` | `driver.updated` | تحديث بيانات السائق |
| `driver.deleted` | `driver.deleted` | حذف السائق |

### **أحداث الوسائط:**

| Topic | EventType | الوظيفة |
|-------|-----------|---------|
| `media.ready` | `media.ready` | إضافة ملف وسائطي جاهز |
| `media.deleted` | `media.deleted` | حذف ملف وسائطي |

### **معالجة الأحداث:**

```go
func (cm *ConsumerManager) handleMessage(ctx context.Context, topic string, msg kafkago.Message) error {
    env, err := events.UnmarshalEnvelope(msg.Value)
    if err != nil {
        return fmt.Errorf("%w: invalid envelope json: %v", pkgKafka.ErrPermanent, err)
    }

    slog.Info("Consuming event for search projection",
        "topic", topic,
        "eventType", env.EventType,
        "eventId", env.EventID)

    switch env.EventType {
    case string(events.DeliveryCreated):
        var p events.DeliveryCreatedPayload
        if err := json.Unmarshal(env.Payload, &p); err != nil {
            return fmt.Errorf("%w: %v", pkgKafka.ErrPermanent, err)
        }
        doc := search.DeliveryDocument{
            DeliveryID: p.DeliveryID,
            CustomerID: p.CustomerID,
            DriverID:   p.DriverID,
            Status:     p.Status,
            Pickup: search.GeoAddress{
                City:    p.Pickup.City,
                Country: p.Pickup.Country,
                Location: search.GeoPoint{
                    Lat: p.Pickup.Location.Lat,
                    Lon: p.Pickup.Location.Lon,
                },
            },
            // ... باقي الحقول
        }
        return cm.indexingService.UpsertDelivery(ctx, doc)

    // ... باقي الأحداث
    }
}
```

### **Dead Letter Queue (DLQ):**

لو في مشكلة في معالجة الحدث، بيتم إرساله إلى DLQ:

```go
dlqProducer := pkgKafka.NewProducer(cfg.KafkaBrokers)

consumer := pkgKafka.NewConsumer(pkgKafka.ConsumerConfig{
    Brokers:    cm.cfg.KafkaBrokers,
    Topic:      topic,
    GroupID:    fmt.Sprintf("%s-%s", cm.cfg.KafkaGroupID, topic),
    MaxRetries: 3,
    DLQ:        dlqProducer,
})
```

---

## Redis Cache

### الاستخدام:

السيرفيس بتستخدم Redis للتخزين المؤقت لنتائج البحث:

```go
func (s *Service) SearchDeliveries(ctx context.Context, q search.DeliverySearchQuery) (search.SearchResult[search.DeliveryDocument], error) {
    cacheKey := s.cache.GenerateKey("deliveries", q)
    var cached search.SearchResult[search.DeliveryDocument]
    if s.cache.Get(ctx, cacheKey, &cached) {
        return cached, nil  // رجع من Cache
    }

    res, err := s.repo.SearchDeliveries(ctx, q)
    if err != nil {
        return search.SearchResult[search.DeliveryDocument]{}, err
    }

    _ = s.cache.Set(ctx, cacheKey, res, 60*time.Second)  // خزّن لمدة 60 ثانية
    return res, nil
}
```

### إنشاء المفاتيح:

```go
func (c *Cache) GenerateKey(prefix string, input interface{}) string {
    b, _ := json.Marshal(input)
    hash := sha256.Sum256(b)
    return fmt.Sprintf("search:%s:%s", prefix, hex.EncodeToString(hash[:]))
}
```

**التنسيق**: `search:deliveries:a1b2c3d4...`

### TTL (Time-To-Live):

- **البحث العادي**: 60 ثانية
- **Autocomplete**: 300 ثانية (5 دقائق)

---

## التكوين

### المتغيرات البيئية:

| المتغير | القيمة الافتراضية | الوصف |
|---------|-------------------|-------|
| `PORT_SEARCH_GRAPHQL` | `4007` | بورت GraphQL |
| `PORT_SEARCH_METRICS` | `9103` | بورت Prometheus Metrics |
| `OPENSEARCH_URL` | `http://opensearch-srv:9200` | عنوان OpenSearch |
| `OPENSEARCH_USERNAME` | `admin` | اسم مستخدم OpenSearch |
| `OPENSEARCH_PASSWORD` | `admin` | كلمة مرور OpenSearch |
| `KAFKA_BROKERS` | `kafka-srv:9092` | Kafka Brokers |
| `KAFKA_GROUP_ID_SEARCH` | `search-service` | Consumer Group ID |
| `REDIS_HOST` | `redis-srv` | عنوان Redis |
| `REDIS_PORT` | `6379` | بورت Redis |
| `SEARCH_CACHE_TTL` | `120` | مدة التخزين المؤقت بالثواني |
| `SEARCH_MAX_PAGE_SIZE` | `100` | أكبر عدد نتائج في الصفحة |
| `SEARCH_MAX_QUERY_LENGTH` | `500` | أكبر طول لنص البحث |

---

## GraphQL

السيرفيس بتوفر GraphQL endpoint على البورت `4007`:

```go
mux := http.NewServeMux()
mux.HandleFunc("/search/graphql", gqlServer.Handler())
mux.HandleFunc("/graphql", gqlServer.Handler())
mux.HandleFunc("/health/live", healthHandler.LivenessHandler())
mux.HandleFunc("/health/ready", healthHandler.ReadinessHandler())
```

### Health Checks:

- `/health/live` - للتحقق إذا السيرفيس شغالة
- `/health/ready` - للتحقق إذا السيرفيس جاهزة تستقبل طلبات

---

## ملخص

search-service هي سيرفيس مهمة في نظام التوصيل في الوقت الحقيقي، مسؤولة عن:

1. **فهرسة البيانات** من الخدمات التانية عبر Kafka
2. **البحث المتقدم** باستخدام OpenSearch مع دعم البحث النصي والجغرافي
3. **التخزين المؤقت** لنتائج البحث باستخدام Redis
4. **إدارة الفهارس** مع دعم إعادة الفهرسة بدون downtime
5. **توثيق الصحة** مع health checks للمراقبة

السيرفيس بتتبع معمارية **Event-Driven** حيث بتستقبل أحداث من الخدمات التانية وtfajrها في OpenSearch، وبتوفر GraphQL API للبحث.
