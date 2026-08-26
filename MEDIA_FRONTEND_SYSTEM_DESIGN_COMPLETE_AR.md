# دليل شامل لنظام Delivery: Media Upload and Processing

**المشروع:** Realtime Delivery microservices  
**الغرض من هذا الملف:** شرح النظام كما هو موجود في المستودع، وشرح العلاقة بين `media-service` و`frontend` وبقية الخدمات، وتوضيح مسؤولية كل طبقة، وتوثيق رحلة رفع وتنزيل وحذف الملفات، ثم تحديد ما هو مطبق فعليًا وما هو جزئي أو غير مثبت.  
**لغة الدليل:** العربية مع الإبقاء على أسماء الملفات والدوال والـ APIs بالإنجليزية حتى يمكن مطابقتها مباشرة مع الكود.  
**ملاحظة مهمة:** هذا الملف توثيق هندسي للمستودع الحالي، وليس وعدًا بأن كل بند في المتطلبات مطبق كاملًا. كل بند مصنف إلى: **مطبق**، **مطبق جزئيًا**، **غير مثبت**، أو **مستبعد عمدًا**.

---

## 1. الخلاصة التنفيذية

النظام عبارة عن منصة Microservices لمعالجة عمليات التسليم والملفات والـ realtime events. الجزء الخاص بالملفات يتمركز حول `media-service` المكتوب بلغة Go، بينما يتعامل `frontend` المكتوب بـ React/TypeScript مع اختيار الملفات، بدء جلسة الرفع، رفع الأجزاء إلى S3/LocalStack، عرض التقدم، استقبال تحديثات WebSocket، والتنزيل والاستكمال. توجد خدمات أخرى مثل `api-gateway` و`user-service` و`notification-service` و`realtime-service` و`search-service`، إضافة إلى البنية التحتية Kafka وNATS وRedis وDynamoDB/S3 وPostgres وOpenSearch.

التصميم المقصود هو عدم تمرير محتوى الملف الكبير عبر الـ API Gateway أو Go HTTP server. الـ Backend ينفذ المصادقة والتفويض والتحقق من الـ metadata وينشئ `uploadId` و`fileId` وPresigned URLs، ثم يرفع المتصفح الملف مباشرة إلى S3. بعد اكتمال الرفع ينشر Backend event إلى Kafka، ويعمل Go worker في الخلفية على الفحص والتحقق من المحتوى واستخراج metadata والضغط وإنشاء renditions مثل WebP وAVIF، ثم يرسل النتيجة إلى DynamoDB وNATS/WebSocket.

> **القرار المعماري الأساسي:** الـ Frontend لا يملك AWS credentials، والـ Backend لا يصبح proxy للـ bytes في الملفات الكبيرة. الصلاحيات وPresigned URLs قصيرة العمر هما الحد الفاصل بين الأمان والأداء.

| المجال | المسؤول الأساسي | المكونات المساندة | النتيجة |
|---|---|---|---|
| هوية المستخدم | `user-service` و`api-gateway` | JWT، shared TS/Go packages | مستخدم موثق |
| بدء الرفع | `media-service` | DynamoDB، Redis، S3 | Upload session |
| نقل bytes | `frontend` مباشرة إلى S3 | Presigned PUT، Multipart Upload | Objects في S3 |
| الفحص والمعالجة | Go workers في `media-service` | ClamAV، FFmpeg، ImageMagick/FFmpeg، Kafka | ملف صالح وrenditions |
| الأحداث | Kafka للأعمال وNATS للـ realtime | shared event packages | انتقال غير متزامن |
| التحديث الفوري | `realtime-service` أو notification path وWebSocket | JWT WebSocket guard | تحديث UI |
| البحث | `search-service` | OpenSearch | فهرسة metadata |
| التشغيل المحلي | Kubernetes/Minikube/Skaffold | LocalStack، Postgres، Redis، Kafka، NATS | بيئة integration |

---

## 2. خريطة المستودع والملفات المهمة

### 2.1 الجذر

يحتوي جذر المستودع على `skaffold.yaml` وملفات Docker وKubernetes و`frontend` و`services` و`packages` و`protos`. ملف Skaffold هو نقطة تشغيل البيئة المحلية: يبني الصور، يضعها في Docker daemon الخاص بـ Minikube، يطبق ملفات `k8s/`، ثم ينتظر readiness للـ Deployments.

| المسار | وظيفته |
|---|---|
| `frontend/` | تطبيق React/TypeScript/Vite للرفع والتنزيل والـ UI والـ WebSocket |
| `services/media-service/` | خدمة Go للـ media lifecycle وPresigned URLs والـ workers |
| `services/api-gateway/` | نقطة الدخول الخارجية وGraphQL/HTTP routing والتحقق الأولي |
| `services/user-service/` | المستخدمون والهوية والصلاحيات وPostgres الخاص بالمستخدمين |
| `services/notification-service/` | الإشعارات، subscriptions، وربط الأحداث بالمستخدمين |
| `services/realtime-service/` | بث realtime events وWebSocket fan-out |
| `services/search-service/` | فهرسة وبحث metadata في OpenSearch |
| `packages/go/` | shared Go packages للمصادقة والأحداث والـ Kafka/NATS والـ logging والـ metrics |
| `packages/ts/` | shared TypeScript packages للـ auth/events/guards/logging/metrics/WebSocket |
| `protos/` | عقود gRPC بين الخدمات |
| `k8s/` | Deployments وServices وSecrets وConfigMaps وPolicies وHPA وPDB |
| `docker-compose.integration.yml` | overlay للتكامل يضيف ClamAV وOTEL Collector وPrometheus وGrafana |

### 2.2 ملف `user.resolver.ts`

الملف الذي وضع فيه هذا الدليل هو `services/user-service/src/modules/user/user.resolver.md`، وهو ملف توثيق جديد بجانب `user.resolver.ts`. تم الحفاظ على ملف TypeScript التنفيذي دون استبداله أو تعديله بهذا الدليل حتى لا يتم استبدال الكود التنفيذي. الـ Resolver في NestJS هو طبقة GraphQL، ويستقبل query أو mutation ثم يستدعي service أو use-case. لا ينبغي أن يحتوي Resolver على منطق S3 أو Kafka أو FFmpeg؛ دوره هو تحويل request إلى service call، وقراءة `CurrentUser`، وتمرير pagination وDTOs، وترك قواعد العمل للـ service.

عند إضافة شرح لكل دالة في Resolver يجب تسجيل أربعة أشياء: اسم العملية، الـ input DTO، الـ guard أو role المطلوب، والـ service method الذي تنتهي إليه العملية. المصادقة ليست مجرد وجود `userId` في input؛ الـ `userId` الصحيح يجب أن يأتي من JWT أو gRPC identity الموثوقة. أي قيمة يرسلها العميل لتحديد owner يجب تجاهلها أو مقارنتها بالهوية المستخرجة من token.

---

## 3. System Design بالتفصيل

### 3.1 الطبقات

يتكون النظام من خمس طبقات مترابطة. الطبقة الأولى هي Edge/API، وتضم Ingress و`api-gateway`. الطبقة الثانية هي domain services، ومنها `media-service` و`user-service` و`notification-service` و`search-service`. الطبقة الثالثة هي messaging، حيث يستخدم Kafka للأحداث التي يجب أن تعيش وتُعاد معالجتها، ويستخدم NATS للتوزيع السريع والخفيف للأحداث اللحظية. الطبقة الرابعة هي storage، وتشمل S3 للـ binary objects وDynamoDB للـ media metadata/state وRedis للـ transient coordination وPostgres للبيانات العلائقية وOpenSearch للبحث. الطبقة الخامسة هي workers وobservability، وتشمل Go worker pool وClamAV وFFmpeg وOTEL وPrometheus/Grafana.

```text
Browser / React Frontend
        |
        v
Ingress -> API Gateway -> Media Service ------------------+
                              |                            |
                 create session / authorize               |
                              |                            |
                  DynamoDB metadata                        |
                              |                            |
                Presigned URL response                     |
                              |                            |
Browser --------------------> S3 / LocalStack              |
                                                           |
S3 event or complete request -> Kafka -> Go Workers --------+
                                         |                  |
                              ClamAV / FFmpeg / metadata    |
                                         |                  |
                              DynamoDB state + S3 renditions
                                         |
                              NATS -> Realtime/WebSocket -> Browser
```

### 3.2 لماذا Kafka وNATS معًا؟

Kafka مناسب للأحداث التي نريد الاحتفاظ بها، partitioning، consumer groups، retry، DLQ، وإعادة replay. مثال ذلك `MEDIA_UPLOADED` و`SCAN_REQUESTED` و`PROCESSING_REQUESTED` و`DELETE_REQUESTED`. NATS مناسب لبث notification منخفض الكمون إلى المشتركين، خصوصًا WebSocket fan-out. لا ينبغي استخدام WebSocket كقناة تنفيذ موثوقة للـ processing؛ هو قناة عرض realtime فقط، بينما الحقيقة الدائمة توجد في DynamoDB/Kafka.

### 3.3 لماذا DynamoDB؟

DynamoDB مناسب لأن حالة الملف تتغير عدة مرات وبمعدل عالٍ، ويمكن تنفيذ atomic conditional writes لمنع سباق `complete` و`delete` أو تكرار Kafka event. يجب أن يحتوي record على owner وfileId وuploadId وstate وprocessingState وtimestamps وversion وchecksum وربما TTL للسجلات المؤقتة. الـ conditional expression هو خط الدفاع ضد انتقالات مثل `READY -> UPLOADING` أو تنفيذ `DELETE` مرتين بطريقة غير متسقة.

### 3.4 لماذا Redis؟

Redis ليس مصدر الحقيقة للـ file ownership. يستخدم للـ rate limits، progress المؤقت، locks قصيرة، idempotency windows، وعدّاد العمليات المتزامنة. إذا اختفى Redis فلا ينبغي أن تصبح ملكية الملف غير معروفة؛ بل يجب أن تبقى في DynamoDB/Postgres. أي lock يجب أن يكون bounded by TTL ويجب ألا يحول Redis إلى نقطة توقف دائمة.

---

## 4. رحلة Authentication وAuthorization

يبدأ المستخدم بتسجيل الدخول عبر مسار الهوية في `user-service` أو المسار الموحد الذي يمر عبر `api-gateway`. يتم إصدار JWT، ثم يرسل العميل token في `Authorization: Bearer` لطلبات HTTP. داخل الخدمة يتم التحقق من signature وissuer وaudience وexpiration وclaims مثل `sub` و`roles` و`permissions`.

في WebSocket لا يكفي قبول `user_id` من query string. التعديل الجديد يستخدم JWT حقيقيًا مع shared package في `packages/go/auth` أو طبقات WebSocket المشتركة في `packages/go/websocket`. الخادم يفك token، يتحقق منه، ثم يبني connection identity من claim موثوق. إذا كان token منتهيًا أو signature غير صحيحة أو لا يملك المستخدم permission المطلوبة، يرفض handshake أو يغلق الاتصال.

| العملية | فحص مطلوب |
|---|---|
| `create upload` | مستخدم authenticated، quota، النوع والحجم والـ permission |
| `presign part` | المستخدم owner أو لديه permission مشاركة، وsession غير منتهية |
| `complete upload` | owner، uploadId صحيح، الأجزاء مكتملة، state يسمح بالانتقال |
| `download` | owner أو ACL/role مسموح، وعدم تسريب object key |
| `delete` | owner أو role إدارية، ثم transition شرطي إلى `DELETING` |
| WebSocket subscription | JWT صالح وsubject يطابق user channel المطلوب |

**النتيجة:** Authentication & Authorization مطبقان في المسار العام، لكن يجب اعتبار اختبارات permission لكل endpoint، واختبار token rotation وrevocation، جزءًا ضروريًا قبل وصف النظام بأنه production-ready بالكامل.

---

## 5. Upload Session وMetadata

### 5.1 إنشاء الجلسة

عندما يختار المستخدم ملفًا، لا يبدأ المتصفح رفع bytes فورًا. يرسل أولًا metadata مثل filename وsize وcontent type وربما checksum. `media-service` ينشئ `fileId` و`uploadId`، يحدد هل العملية single PUT أم multipart، يحفظ record في DynamoDB بحالة `INITIATED`، ثم يعيد بيانات الرفع.

الـ response قد يحتوي على URL مباشر للملف الصغير أو معلومات multipart مثل `uploadId` وpart size وURLs لكل جزء أو endpoint لتوقيع الأجزاء عند الطلب. يجب عدم إصدار URLs بلا expiration أو بلا ربط بالـ object key الصحيح. يجب أن يكون object key غير قابل للتلاعب من العميل، مثل prefix مرتبط بالـ tenant/user وfileId عشوائي.

### 5.2 Validation Before Upload

قبل إصدار URL يجب التحقق من الحجم الأقصى، extension المسموح، MIME المعلن، quota، user permission، IP/user rate limit، وعدد الـ uploads المتزامنة. هذه validation تحسين أمني وأدائي لكنها ليست ثقة نهائية؛ لأن `Content-Type` واسم الملف يمكن تزويرهما.

| تحقق أولي | مصدره | هل يكفي وحده؟ |
|---|---|---|
| الحجم | request metadata | لا، يجب مقارنة الحجم الفعلي بعد الرفع |
| extension | filename | لا، يمكن تزوير الاسم |
| MIME | browser header | لا، يجب فحص magic bytes |
| quota | DynamoDB/Redis | يحتاج conditional reservation تحت concurrency |
| permission | JWT وACL | نعم فقط بعد التحقق من token والـ ownership |
| rate limit | Redis أو shared limiter | لا يغني عن quota أو authorization |

### 5.3 Multipart Upload

الملف الكبير يقسم إلى parts مستقلة. الـ Frontend يرفع 4–8 أجزاء بالتوازي حسب الجهاز والشبكة، ويسجل ETag لكل جزء. عند فشل جزء، يعاد ذلك الجزء فقط مع backoff وjitter. عند refresh أو reconnect، يستدعي العميل resume endpoint أو يستعيد `uploadId` من IndexedDB، ثم يستعلم عن الأجزاء المكتملة ويكمل الناقص.

الحالة الصحيحة تكون تقريبًا:

```text
INITIATED -> UPLOADING -> UPLOADED -> SCANNING -> PROCESSING -> READY
                  |             |          |            |
               PAUSED        FAILED     QUARANTINED  FAILED
                  |
               CANCELED
```

لا يجوز تنفيذ `complete` إلا إذا كان server قادرًا على التأكد من قائمة الأجزاء والـ ETags. لا ينبغي قبول قائمة أجزاء مرسلة من العميل دون مقارنتها بحالة S3 أو نتيجة `ListParts`.

---

## 6. S3 وPresigned URLs

S3 يخزن bytes، وليس مصدر authorization. الـ Backend يعطي `Presigned PUT` أو multipart presigned URLs بمدة قصيرة. المتصفح يتعامل مع الرابط فقط، ولا يرى AWS access key أو secret key. بعد الرفع يجب أن يكون object key والـ bucket معروفين للـ Backend، وألا يسمح العميل بكتابة خارج prefix المخصص.

في البيئة المحلية يستخدم LocalStack كمحاكاة لـ S3 وDynamoDB. نجاح pod لا يعني بالضرورة أن كل API داخل LocalStack جاهز؛ لذلك يجب اختبار `/_localstack/health`، وإنشاء bucket، وإنشاء table، وتنفيذ PutObject وHeadObject وListParts من داخل cluster.

### S3 Lifecycle

المطلوب أن يحذف S3 incomplete multipart uploads وtemporary objects بعد مدة. هذا بند **غير مثبت بالكامل** ما لم يوجد كود أو IaC يطبق lifecycle configuration ويُختبر عبر LocalStack/AWS. لا يكفي وجود كلام في README. التحقق الحقيقي هو `GetBucketLifecycleConfiguration` ثم إنشاء multipart غير مكتمل والانتظار أو محاكاة lifecycle، مع تسجيل النتيجة.

### DynamoDB TTL

TTL يستخدم للـ temporary upload sessions أو idempotency records أو progress records. TTL ليس deletion فوريًا؛ DynamoDB يحذف لاحقًا. لذلك يجب ألا يعتمد business logic على اختفاء السجل في ثانية محددة. التحقق يكون بقراءة table description والتأكد من أن TTL مفعّل على attribute محدد، ثم اختبار record تجريبي. هذا أيضًا **يحتاج إثباتًا فعليًا** منفصلًا عن مجرد تعريف field في Go.

---

## 7. Post-upload Validation وMalware Scanning

بعد اكتمال الرفع، ينتقل الملف إلى `UPLOADED` ثم `SCANNING`. worker يقرأ object من S3 أو يمرره إلى ClamAV. يجب فحص actual bytes وmagic bytes وليس MIME القادم من المتصفح فقط. إذا كان الملف ضارًا أو لا يطابق النوع المتوقع، يتحول إلى `QUARANTINED` ولا يصدر له Download URL صالح للمستخدم.

يجب أيضًا وضع حدود عملية للفحص: maximum scan size، timeout، memory، وفشل ClamAV. عند فشل dependency لا يصح جعل الملف `READY`؛ الأفضل إبقاؤه في retryable state أو `SCAN_FAILED` مع event وmetrics. إذا كان ClamAV غير متاح في التشغيل الحالي، يجب أن يفشل الإطلاق الآمن closed بدل bypass غير مقصود.

---

## 8. Compression وImage Processing وVideo Processing

### 8.1 Compression

الضغط يطبق فقط على أنواع مناسبة، مع الحفاظ على original عندما يكون مطلوبًا قانونيًا أو وظيفيًا. يجب ألا يستبدل worker الملف الأصلي قبل نجاح النسخة الجديدة والتحقق منها. كل rendition تحتاج object key مستقل وmetadata يربطها بالـ original.

### 8.2 Image Worker

المسار الأساسي هو `services/media-service/internal/workers/image/worker.go`. التعديل الجديد أضاف FFmpeg-backed rendition generation لدعم WebP وAVIF، مع تحديث MIME whitelist. على worker قراءة object، التحقق من نوعه، تشغيل التحويل بحدود timeout، فحص exit code، والتحقق من وجود output قابل للقراءة، ثم رفع rendition إلى S3 وتحديث DynamoDB.

| rendition | الاستخدام | ملاحظة تشغيلية |
|---|---|---|
| original | المصدر الأصلي | لا يحذف إلا بسياسة صريحة |
| thumbnail | preview سريع | حجم صغير وcache مناسب |
| WebP | متصفح حديث وتقليل bandwidth | يجب ضبط MIME `image/webp` |
| AVIF | ضغط أعلى عند دعم المتصفح | يجب ضبط MIME `image/avif` |
| sizes متعددة | responsive UI | يجب حفظ width/height في metadata |

وجود كود التحويل لا يثبت وحده نجاح FFmpeg في cluster. يجب اختبار binary داخل image worker، ووجود codec، ومساحة `/tmp`، والـ CPU limit، ونتيجة الملف فعليًا.

### 8.3 Video Worker

`services/media-service/internal/workers/video/worker.go` مسؤول عن FFmpeg للفيديو. المتطلبات المذكورة تشمل resolutions وHLS versions وduration وcodec. الموجود يجب تصنيفه بدقة حسب ما تنفذه الدوال والـ Docker image. لا نعد بأن HLS كامل مطبق إلا إذا وجدت playlist وsegments واختبار playback. FFmpeg worker يجب أن يستخدم subprocess timeout، يمنع path traversal، ويكتب في scratch directory مع cleanup.

### 8.4 Metadata Extraction

`services/media-service/internal/workers/metadata/worker.go` يستخرج dimensions وduration وcodec وresolution وغيرها. metadata الناتجة يجب أن تكون server-derived، لا من claims العميل. يجب حفظها مع version من extractor، لأن اختلاف FFmpeg/ImageMagick قد يعطي نتائج مختلفة.

---

## 9. Kafka، Retry، Backoff، وDLQ

Kafka ينشر business events، وshared event contracts موجودة في `packages/go/events` و`packages/ts/src/events`. يجب أن يحتوي envelope على event ID وevent type وtrace ID وoccurred-at وربما schema version. `packages/go/kafka/kafka.go` يوفر serialization/envelope logic، ويجب استخدامه بدل إعادة تعريف envelope في كل خدمة.

الأحداث المقترحة هي `MEDIA_UPLOADED` و`SCAN_REQUESTED` و`PROCESSING_REQUESTED` و`MEDIA_READY` و`DELETE_REQUESTED` ونتائج مثل `DELETE_COMPLETED` و`DELETE_FAILED`. consumer يجب أن يكون idempotent؛ event ID نفسه أو file version نفسه لا ينفذ side effect مرتين.

### Retry + exponential backoff + jitter

عند فشل مؤقت، ينتظر worker فترة مثل:

```text
backoff = min(maxDelay, baseDelay * 2^attempt) + random(0, jitter)
```

الـ jitter يمنع أن تعيد مجموعة workers المحاولة في اللحظة نفسها. لا ينبغي retry أخطاء validation الدائمة مثل unsupported type بلا تغيير input. يجب تمييز transient failure عن permanent failure.

### Dead Letter Queue

بعد استنفاد retries يذهب الحدث إلى DLQ مع original payload وevent ID وattempt count وlast error وtrace ID وfailed-at. التعديل الجديد أضاف Kafka DLQ logic. لكن اختبار DLQ الحقيقي يحتاج تشغيل Kafka، إرسال event يفشل عمدًا، التحقق من عدد المحاولات، قراءة DLQ، ثم replay آمن بعد إصلاح السبب.

---

## 10. Atomic State Transitions وIdempotency

كل transition مهم يجب أن يكون conditional في DynamoDB. مثال: `UPLOADED -> SCANNING` بشرط أن تكون الحالة الحالية `UPLOADED`، و`DELETING -> DELETED` بشرط أن يكون version المتوقع صحيحًا. إذا وصل حدث duplicate أو request متزامن، يفشل الشرط الثاني بدون تخريب الحالة.

| العملية | مفتاح idempotency | السلوك المتوقع عند التكرار |
|---|---|---|
| create session | client request ID أو deterministic hash | يعيد session الأصلية ضمن window |
| complete upload | `fileId + uploadId` | لا يعيد complete أو event side effect |
| delete | `fileId + operation ID` | لا يحذف object مرتين |
| process event | Kafka event ID | acknowledge duplicate بعد no-op آمن |
| presign | part number + uploadId | يعيد URL جديدًا أو يستفيد من الحالة دون تجاوز الصلاحيات |

الاختبار المطلوب هو تشغيل عشرات أو مئات goroutines على نفس transition ونفس event، ثم التأكد من وجود record نهائي واحد، وevent side effect واحد، وعدم تجاوز quota.

---

## 11. Delete Workflow وCancel Upload

الحذف ليس `DeleteObject` مباشرًا من المتصفح. العميل يرسل `DELETE /media/files/:id` أو GraphQL mutation إلى Backend. Backend يتحقق من ownership، يحول الحالة شرطيًا إلى `DELETING`، وينشر `DELETE_REQUESTED`. worker يحذف original وrenditions وtemporary objects ثم يكتب `DELETED` وينشر `DELETE_COMPLETED`.

إذا فشل S3، يعاد الحذف. المطلوب idempotency لأن نجاح حذف object مرتين يجب ألا يجعل الحالة inconsistent؛ S3 delete غالبًا idempotent، لكن metadata event ليس كذلك تلقائيًا. عند إلغاء multipart upload، يجب تنفيذ AbortMultipartUpload، وتحديث session إلى `CANCELED`، وإلغاء retry loops، ومنع complete بعد الإلغاء.

---

## 12. WebSocket وRealtime Fan-out

الـ WebSocket يرسل progress وstate updates للواجهة. يجب أن يكون payload صغيرًا ويحتوي fileId وuploadId وstate وprogress وtrace/request correlation عند الحاجة. لا يجب إرسال secrets أو presigned URLs طويلة العمر عبر logs أو events عامة.

المسار النموذجي هو: worker ينشر NATS event، realtime/notification service يشترك، ثم يرسل إلى WebSocket connections الخاصة بالمستخدم. عند disconnect لا تضيع الحقيقة؛ عند reconnect يقرأ Frontend الحالة الحالية من API ثم يستقبل updates الجديدة. التعديل الجديد أصلح URL resolution للـ ingress حتى لا يفترض المتصفح `localhost` أو port داخلي.

**JWT WebSocket auth مطبق كتعديل حقيقي** بدل الثقة في `user_id` من query/header غير موثق. ومع ذلك، يجب اختبار: token صحيح، token منتهي، user يحاول الاشتراك في قناة user آخر، غياب token، token signed بمفتاح آخر، reconnect، وrevocation.

---

## 13. Frontend ومسؤولياته

### 13.1 File Picker وclient validation

الواجهة تسمح باختيار الصور والفيديو والأنواع المسموحة. تفحص الحجم والامتداد وMIME لتحسين تجربة المستخدم، لكن هذه ليست validation أمنية نهائية. يجب أن تعرض سبب الرفض بوضوح، وتمنع بدء requests غير لازمة.

### 13.2 Upload queue وparallelism

الواجهة يمكن أن تدير عدة ملفات، لكن concurrency يجب أن يكون محدودًا. لكل ملف state محلي: `queued`, `initiated`, `uploading`, `paused`, `failed`, `uploaded`, `scanning`, `processing`, `ready`, `canceled`. لا ينبغي أن تتعارض queue المحلية مع state server؛ عند reconnect يفضل reconciliation.

### 13.3 Resumable upload

يحفظ العميل `fileId` و`uploadId` وpart size وETags والأجزاء المكتملة في IndexedDB أو local storage. بعد refresh يستدعي endpoint resume، ويتجنب إعادة رفع الأجزاء الموجودة. يجب عدم حفظ presigned URLs لفترة أطول من expiration بدون إعادة توقيعها.

المتطلبات المكتملة جزئيًا حتى الاختبار الحقيقي:

| الوظيفة | تقييم |
|---|---|
| تقسيم الملف | موجود في المسار الأمامي بحسب helper/config |
| parallel parts | موجود كتصميم ويحتاج قياس concurrency |
| retry للجزء | موجود/مطلوب تأكيد integration |
| pause | يحتاج اختبار أن الأجزاء الجديدة تتوقف فقط |
| cancel | يحتاج اختبار Abort فعلي في S3/LocalStack |
| resume بعد انتهاء URLs | أضيف helper/logic، ويجب تشغيل اختبار live يطلب URLs جديدة |

### 13.4 Range/Resume Download

التعديل الجديد أضاف resumable download helper يعتمد HTTP Range. عند استمرار التنزيل يجب إرسال `Range: bytes=start-` والتحقق من `206 Partial Content` و`Content-Range`، ثم append للملف لا overwrite. يجب اختبار CORS على `Content-Range`, `Accept-Ranges`, `Content-Length` وheaders اللازمة. إذا كان التنزيل من Presigned S3، يجب أن يدعم S3/LocalStack range semantics؛ وإذا مر عبر Backend، يجب ألا يتحول Backend إلى bottleneck بلا سبب.

### 13.5 WebSocket URL resolution

الواجهة يجب أن تبني WebSocket URL من `window.location` أو ingress-aware config، فتستخدم `wss` عند HTTPS، وتوجه إلى المسار الخارجي الصحيح بدل `ws://localhost` أو service DNS داخلي. هذا إصلاح مهم عند النشر خلف Ingress.

---

## 14. العلاقة مع Google Drive وYouTube وDropbox

هذه الخدمات ليست جزءًا تلقائيًا من S3 upload pipeline. العلاقة الصحيحة تكون عبر connector أو integration منفصل: المستخدم يوافق OAuth، يحصل Backend على access/refresh token محفوظ آمنًا، ثم يبدأ import/export job. لا يجوز أن يضع Frontend secrets أو tokens طويلة العمر.

| النظام الخارجي | العلاقة المحتملة |
|---|---|
| Google Drive | Import ملف إلى media-service أو export rendition إلى Drive باستخدام Google Drive API |
| YouTube | ليس storage عامًا للملف الداخلي؛ يمكن أن يكون publish/export video workflow بعد permission واضحة |
| Dropbox | Import/export عبر Dropbox API، مع mapping بين provider file ID وinternal fileId |

يجب أن يظل `media-service` صاحب state الداخلي. يمكن حفظ `externalProvider`, `externalObjectId`, `syncState`, `lastSyncedAt` و`permission snapshot`. لا يجب جعل Google Drive أو YouTube أو Dropbox مصدر authorization لملف داخلي إلا بعد مزامنة صلاحيات صريحة. هذه التكاملات **غير مثبتة في الكود الحالي ما لم توجد connectors وOAuth handlers وworkers واختبارات مستقلة**. لا نخلطها مع CDN؛ الـ CDN مستبعد عمدًا من هذا الإصدار.

---

## 15. Observability: Logging وMetrics وTracing

كل عملية مهمة يجب أن تربط `requestId`, `traceId`, `userId`, `fileId`, `uploadId`, `eventId` و`attempt`. shared logging packages في `packages/go/logging` و`packages/ts/src/logging` تساعد على عدم إعادة تعريف الحقول في كل خدمة.

التعديل الجديد أضاف OTLP gRPC tracing exporter. المسار المقصود هو API span ثم Kafka producer span ثم worker consumer span ثم S3/ClamAV/FFmpeg spans ثم NATS/WebSocket span. يجب تمرير trace context في Kafka envelope أو headers، وعدم إنشاء trace جديد يقطع السلسلة.

| metric | سبب القياس |
|---|---|
| upload bytes/sec | قياس الأداء الحقيقي |
| upload failures | كشف أخطاء S3/network |
| presign latency | قياس API path |
| processing duration | كشف بطء FFmpeg/ClamAV |
| retries وDLQ count | صحة الاعتمادية |
| Kafka consumer lag | ضغط الخلفية وتأخر المستخدم |
| S3 errors | صحة التخزين |
| WebSocket connections | capacity وfan-out |
| CPU/memory | ضبط requests/limits وHPA |
| p95/p99 latency | SLO وليس المتوسط فقط |

تمت إضافة Kafka consumer lag metrics وPrometheus alert rules. لا يكفي وجود ملفات metrics؛ يجب التأكد من أن Prometheus يكتشف targets، وأن Grafana dashboard يقرأ series فعلية، وأن alert يطلق في test condition.

---

## 16. Kubernetes وSkaffold

### 16.1 الموارد

ملفات `k8s/media-depl.yaml` و`k8s/notification-db-depl.yaml` و`k8s/user-db-depl.yaml` تحتوي على Deployments وServices، وبعضها HPA وPDB وNetworkPolicy. `media-service` يحتاج DynamoDB/S3 وKafka، لذلك يمكن أن يبقى في startup loop حتى تصبح LocalStack APIs جاهزة.

### 16.2 مشكلة Postgres التي تم إصلاحها

كان تعريف PVC موجودًا، لكن الـ Deployment كان يركب `emptyDir` بدل PVC. هذا يجعل التخزين غير دائم، ويجعل وجود PVC لا يفيد الحاوية. تم تعديل `notification-db-depl.yaml` و`user-db-depl.yaml` لربط volume باسم `postgres-storage` بـ `notification-db-pvc` و`user-db-pvc`.

تم أيضًا تصحيح فحص البيانات غير المكتملة ليتوافق مع `PGDATA=/var/lib/postgresql/data/pgdata`، وجعل `pg_isready` يستخدم `$POSTGRES_USER` بدل افتراض مستخدم قد لا يطابق secret. تم التحقق من صحة YAML عبر `kubectl apply --dry-run=server`.

### 16.3 LocalStack

LocalStack image كبيرة جدًا، وظهر في التشغيل أن تحميل `localstack/localstack:3.8.0` أخذ وقتًا طويلًا، ثم أصبح pod `Ready`. لذلك كانت بعض أخطاء `connection refused` ناتجة عن startup dependency وليس compile failure. يجب تحميل الصور الثقيلة مسبقًا في Minikube أو استخدام registry داخلي/캐ش مناسب في CI.

### 16.4 نتيجة آخر تشغيل

في آخر تشغيل controlled لـ `skaffold dev --cache-artifacts=true --build-concurrency=0` أصبحت Kafka وNATS وRedis وOpenSearch وLocalStack جاهزة، بينما بقيت database images في مرحلة السحب/التهيئة في نافذة الانتظار، وبقي `media-service` يعيد محاولة DynamoDB حتى يصبح API مستقرًا. لذلك لا يصح تسجيل النتيجة على أنها **full rollout verified** بعد؛ الصحيح أنها **infrastructure mostly stabilized مع بقاء verification النهائي بعد جاهزية Postgres وmedia health**.

أمر التشغيل المطلوب هو:

```powershell
skaffold dev --cache-artifacts=true --build-concurrency=0
```

خطوات التشخيص الصحيحة:

```powershell
kubectl --context minikube get pods
kubectl --context minikube get events --sort-by=.lastTimestamp
kubectl --context minikube describe pod <pod>
kubectl --context minikube logs <pod> --tail=200
kubectl --context minikube get pvc
kubectl --context minikube get svc
```

لا ينبغي اعتبار `Running` مساويًا لـ `Ready`. يجب قراءة `READY`, startup probe, readiness probe، وlogs.

---

## 17. اختبار التكامل المطلوب

يجب أن تكون اختبارات التكامل خارج unit tests وتعمل مع Kafka وNATS وS3/DynamoDB وClamAV وFFmpeg وWebSocket وPostgres. كل test يحتاج cleanup وunique namespace أو unique IDs حتى لا يتأثر بنتيجة اختبار سابق.

| السيناريو | خطوات الإثبات |
|---|---|
| Kafka/DLQ | event يفشل عمدًا، retries، DLQ، replay، no duplicate side effect |
| NATS/WebSocket | اتصال JWT صحيح، publish event، وصول user المقصود فقط |
| S3/DynamoDB | create table/bucket، create session، upload، complete، state transitions |
| ClamAV | ملف harmless يصبح scanned/ready، EICAR يصبح quarantined |
| FFmpeg | صورة تنتج WebP/AVIF، فيديو ينتج rendition، metadata صحيحة |
| Resume upload | رفع بعض الأجزاء، انتهاء URLs، توقيع جديد، استكمال الأجزاء الناقصة |
| Range download | الطلب الثاني يعيد 206 وContent-Range ويستكمل bytes بلا تكرار |
| idempotency | duplicate create/complete/delete/event تحت concurrency |
| lifecycle/TTL | قراءة إعدادات S3 lifecycle وDynamoDB TTL ثم اختبار record تجريبي |

اختبار EICAR يستخدم فقط في بيئة اختبار معزولة ولا يوضع في production bucket. كل integration test يجب أن يسجل trace ID كي يمكن تتبعه في Jaeger/OTEL.

---

## 18. Performance وScalability

لا يكفي قول إن النظام scalable بسبب وجود HPA أو Kafka. يجب القياس. السيناريوهات الرئيسية هي upload صغير، multipart كبير، processing image، processing video، concurrent downloads، duplicate events، وWebSocket fan-out.

| القياس | طريقة القياس |
|---|---|
| throughput | عدد الملفات أو bytes في الثانية |
| p50/p95/p99 | زمن create، presign، complete، processing، download |
| Kafka lag | lag لكل consumer group أثناء الحمل |
| CPU/memory | container metrics وworker subprocesses |
| S3 latency/error | histogram وerror counter |
| DB conditional failures | عدد race conflicts المتوقعة |
| WebSocket fan-out | عدد الاتصالات والرسائل/ثانية |
| queue depth | pending jobs وprocessing time |

يجب تشغيل load test تدريجيًا: smoke، ثم 10 مستخدمين، ثم 50، ثم الزيادة حتى تقترب الموارد من limits. لا يرفع concurrency بلا حدود؛ يجب أن يكون `worker pool size` و`multipart concurrency` و`per-user quota` و`Redis rate limit` متناسقة. HPA يعتمد على metrics حقيقية، وPDB يحمي availability عند eviction لكنه لا يصلح service لا يبدأ أصلًا.

---

## 19. Security checklist

يجب تدوير JWT signing keys، والتحقق من issuer/audience، وعدم تسجيل tokens أو presigned URLs كاملة. يجب منع path traversal وsymlink attacks في worker scratch directories، وتعطيل shell interpolation غير الآمن، ووضع timeout لكل FFmpeg وClamAV وS3 call. يجب جعل NetworkPolicies أقل صلاحية ممكنة، واستخدام Secrets بدل hard-coded credentials.

يجب أيضًا التعامل مع zip bombs وdecompression bombs، وتحديد limits للـ image dimensions والفيديو duration، ومنع الملفات التي تحمل extension مسموحًا لكن magic bytes مختلفة. بعد quarantine يجب ألا يسمح endpoint download بإصدار URL.

---

## 20. حالة التحقق من البنود المطلوبة

| البند | الحالة الحالية | ما يلزم لإغلاقه نهائيًا |
|---|---|---|
| JWT WebSocket auth | **مطبق** | integration tests للحالات السلبية |
| OTLP tracing exporter | **مطبق** | إثبات spans في collector/backend |
| WebP/AVIF | **مطبق في الكود** | اختبار output داخل worker image |
| Kafka/DLQ integration | **موجود جزئيًا** | تشغيل test live وإثبات replay |
| Resume upload بعد انتهاء URLs | **helper موجود** | test live مع انتهاء URL فعلي |
| Range download | **helper موجود** | إثبات 206 وContent-Range وCORS |
| conditional writes concurrency | **مطبق** | benchmark/test race عالي التوازي |
| duplicate Kafka events | **مطبق تصميميًا** | test duplicate event فعلي |
| S3 lifecycle | **غير مثبت بالكامل** | apply/get config واختبار cleanup |
| DynamoDB TTL | **غير مثبت بالكامل** | تفعيل TTL والتحقق من table |
| metrics وKafka lag | **مطبق** | targets/dashboard/alert smoke test |
| performance p95/p99 | **غير مكتمل** | تشغيل load suite وحفظ النتائج |
| CDN | **مستبعد عمدًا** | لا يتم تنفيذه في هذا الإصدار |
| Google Drive | **غير مثبت** | OAuth connector وimport/export tests |
| YouTube | **غير مثبت** | publish workflow منفصل إن كان مطلوبًا |
| Dropbox | **غير مثبت** | OAuth connector وsync workers |

---

# القسم الأخير: كل الإضافات الجديدة التي ظهرت في هذه المحادثة

هذا القسم مستقل عمدًا ويجمع التغييرات الجديدة التي أضيفت أو تم تنفيذها أثناء المحادثة الحالية، مع توضيح أثر كل تغيير وطريقة التحقق منه.

## A. JWT حقيقي داخل WebSocket

تم استبدال الثقة غير الآمنة في `user_id` القادم من query أو header غير موثق بمسار JWT حقيقي. WebSocket الآن يحتاج token صالحًا، ويستخرج identity من claims بعد التحقق من signature وexpiration، ثم يربط الاتصال بالمستخدم الحقيقي. هذا يمنع أن يطلب مستخدم updates تخص مستخدمًا آخر بمجرد تغيير query parameter.

**التحقق المطلوب:** فتح اتصال token صحيح، ثم token منتهٍ، ثم token بتوقيع خاطئ، ثم محاولة الاشتراك في قناة مستخدم آخر. يجب أن يفشل الأخير، ويجب ألا يظهر أي connection unauthorized في fan-out.

## B. OTLP gRPC tracing exporter

كان tracing context موجودًا دون exporter فعلي. أضيف exporter عبر OTLP gRPC ليرسل spans إلى OpenTelemetry Collector. يجب أن يبدأ trace من API، ويحافظ Kafka envelope أو headers على context، ثم يكمل داخل worker وعمليات S3 وFFmpeg وWebSocket.

**التحقق المطلوب:** تشغيل collector، تنفيذ upload، البحث عن trace واحد له spans عبر الخدمات. إذا ظهر span منفصل بلا parent فهذا يعني أن propagation ناقص.

## C. WebP وAVIF processing

تم تحديث `services/media-service/internal/workers/image/worker.go` لإنتاج renditions عبر FFmpeg، مع دعم WebP وAVIF وتحديث MIME whitelist. يجب أن تكون outputs ذات `image/webp` و`image/avif` وأن تحفظ metadata للعرض.

**التحقق المطلوب:** رفع JPEG/PNG صالح، انتظار `READY`، تنفيذ `HeadObject` لكل rendition، فحص MIME والأبعاد، ثم تنزيل output وفتح الصورة فعليًا.

## D. Kafka DLQ وexponential backoff مع jitter

تمت إضافة retry behavior يعتمد exponential backoff مع jitter، وبعد عدد محاولات محدد يتم إرسال الحدث إلى DLQ. تمت مراعاة event identity والـ error context حتى يمكن التحقيق وreplay.

**التحقق المطلوب:** إيقاف dependency أو حقن failure مؤقت، مراقبة retries، التأكد من زيادة التأخير، قراءة DLQ، ثم replay بعد إصلاح dependency دون تنفيذ side effect مرتين.

## E. Atomic state transitions عبر DynamoDB conditional writes

تم فرض انتقالات الحالة بشروط DynamoDB، بحيث لا يستطيع request أو event متزامن تغيير الحالة إذا لم تكن الحالة الحالية مناسبة أو إذا تغير version. هذا يعالج race conditions بين complete وdelete وبين duplicate workers.

**التحقق المطلوب:** تشغيل concurrent complete/delete/process على نفس record، وفحص أن transition واحدة فقط تنجح وأن الـ rejected operations لا تنشئ objects أو events إضافية.

## F. Kafka consumer lag metrics وPrometheus alerts

تمت إضافة قياسات lag للمستهلكين وقواعد alert. هذه الإضافة تجعل الضغط على worker pool ظاهرًا بدل أن يختبئ خلف بطء UI.

**التحقق المطلوب:** إيقاف consumer أو إرسال backlog، التأكد من ارتفاع lag، ظهور metric في Prometheus، ثم اشتغال alert عند تجاوز threshold وعودة metric بعد التعافي.

## G. Docker Compose integration overlay

تم إنشاء `docker-compose.integration.yml` ليشمل ClamAV وOTEL Collector وPrometheus وGrafana بجانب dependencies المحلية. الغرض هو جعل اختبار scanning وtracing وmetrics قريبًا من تشغيل حقيقي بدل unit mocks فقط.

## H. Resumable download باستخدام HTTP Range

أضيف helper للتنزيل القابل للاستكمال. يجب أن يبدأ من byte offset المحفوظ، ويرسل Range request، ويتحقق من `206 Partial Content` و`Content-Range`، ثم يضيف الجزء الجديد إلى الملف المحلي. يجب التأكد من CORS expose headers، وإلا فلن يستطيع browser JavaScript قراءة Content-Range.

## I. WebSocket URL resolution خلف Ingress

تم إصلاح بناء WebSocket URL حتى يعتمد protocol وhost والمسار الخارجي، ويستخدم `wss` عندما تكون الصفحة HTTPS. هذا يمنع فشل الاتصال عند نشر Frontend خلف Ingress حيث لا يرى المتصفح service DNS الداخلي.

## J. إصلاح Kubernetes Postgres storage

كان كل Deployment يعرّف PVC، لكنه يستخدم `emptyDir` فعليًا. تم تحويل mount إلى PVC الصحيح في `notification-db-depl.yaml` و`user-db-depl.yaml`. كما تم تصحيح فحص `PGDATA` ليستخدم `pgdata/PG_VERSION` و`pgdata/global/pg_control`، وجعل probes تستخدم `$POSTGRES_USER`.

**أثر الإصلاح:** أصبح storage المعلن هو storage المستخدم، وتقل احتمالات فقد البيانات أو إعادة initialization غير المقصودة، وأصبحت probes متوافقة مع إعدادات Postgres.

## K. تشخيص Skaffold النهائي في هذه المحادثة

تم تشغيل الأمر المطلوب:

```powershell
skaffold dev --cache-artifacts=true --build-concurrency=0
```

ظهرت أولًا مشاكل readiness بسبب بطء تحميل LocalStack وصور Postgres، وليس بسبب compilation. LocalStack image بحجم كبير استغرق وقتًا طويلًا، ثم أصبحت Kafka وNATS وRedis وOpenSearch وLocalStack جاهزة في التشغيل اللاحق. بقيت Postgres images في نافذة pull/initialization، وكان media-service ينتظر DynamoDB بينما LocalStack كان يستقر. لذلك تم تنفيذ preload لـ LocalStack وPostgres، وتعديل manifests، ثم إعادة التشغيل. **التحقق الكامل النهائي يحتاج تشغيل Skaffold في جلسة تستمر حتى تصبح كل الـ Deployments Ready، ثم تنفيذ smoke/integration tests المذكورة أعلاه.**

## L. ما لم يتم تنفيذه عمدًا

لم يتم تنفيذ CDN بناءً على طلب صريح. لذلك التنزيل الحالي يعتمد على Presigned GET URLs أو مسار Backend الموجود، ولا توجد طبقة CDN أو cache invalidation أو edge distribution في هذا الإصدار. Google Drive وYouTube وDropbox ليست جزءًا من هذا الإصدار إلا إذا كان لها connector وكود OAuth وworker واضح داخل المستودع.

---

## 21. قائمة التشغيل العملية من الصفر

1. تأكد من تشغيل Docker وMinikube واستخدام context الصحيح.
2. افحص وجود `secrets` و`notification-secrets` و`media-service-secrets` وعدم احتوائها على values غير صالحة.
3. حمّل الصور الثقيلة مسبقًا في Minikube عند ضعف الشبكة:

```powershell
minikube image load localstack/localstack:3.8.0 --profile minikube
minikube image load postgres:16 --profile minikube
```

4. شغل:

```powershell
skaffold dev --cache-artifacts=true --build-concurrency=0
```

5. انتظر حتى يصبح كل Deployment `READY`، ولا تعتمد على أن pod في `Running` فقط.
6. اختبر health endpoints وDynamoDB/S3/ClamAV وKafka/NATS.
7. نفذ upload صغير ثم multipart كبير.
8. افحص state transitions وtrace وmetrics.
9. نفذ resume upload بعد انتهاء presigned URL.
10. نفذ range download وتحقق من headers.
11. نفذ delete وراقب `DELETE_COMPLETED` أو `DELETE_FAILED`.
12. شغل load test واحفظ p95/p99 وconsumer lag وCPU/memory.

---

## 22. مراجع تقنية

[1]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/mpuoverview.html "Amazon S3 Multipart Upload Overview"

[2]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/PresignedUrlUploadObject.html "Amazon S3 Presigned URLs"

[3]: https://docs.aws.amazon.com/amazons3/latest/userguide/object-lifecycle-mgmt.html "Amazon S3 Object Lifecycle Management"

[4]: https://docs.aws.amazon.com/amazons3/latest/userguide/mpu-abort-incomplete-mpu-lifecycle-config.html "Abort Incomplete Multipart Uploads"

[5]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/TTL.html "DynamoDB Time to Live"

[6]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/ "Kubernetes Persistent Volumes"

[7]: https://opentelemetry.io/docs/concepts/signals/traces/ "OpenTelemetry Traces"

[8]: https://kafka.apache.org/documentation/ "Apache Kafka Documentation"

[9]: https://ffmpeg.org/documentation.html "FFmpeg Documentation"

[10]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Range_requests "HTTP Range Requests"

[11]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/206 "HTTP 206 Partial Content"

[12]: https://www.clamav.net/documents/ "ClamAV Documentation"

[13]: https://jwt.io/introduction "JSON Web Tokens Introduction"

---

**الخلاصة النهائية:** الأساس المعماري المطلوب موجود بدرجة قوية، والتعديلات الجديدة الأساسية الخاصة بـ JWT WebSocket وOTLP وWebP/AVIF وDLQ وconditional writes وlag metrics وrange/resume helpers وPostgres PVC تم توثيقها هنا. لكن وصف النظام بأنه مكتمل production-grade يحتاج إغلاق الاختبارات الحية الخاصة بـ LocalStack وPostgres وClamAV وFFmpeg وS3 lifecycle وDynamoDB TTL وperformance p95/p99، مع بقاء CDN مستبعدًا عمدًا.


---

# ملحق تدقيق الكود الحالي (تمت مراجعته من الملفات الفعلية)

هذا الملحق يفرّق بين **ما ينفذه الكود الآن** وبين ما يرد كتصميم مستهدف أو توصية تشغيلية. الاعتماد هنا على قراءة الملفات الفعلية داخل المستودع، وليس على أسماء المتطلبات فقط.

## A. المسار الفعلي لإنشاء جلسة الرفع

نقطة التشغيل الأساسية هي `services/media-service/cmd/server/main.go`. الدالة `main()` تنشئ الاتصالات مع DynamoDB وS3/LocalStack وRedis وNATS وKafka، تفعّل TTL وLifecycle الخاصة بـ S3 عند نجاح الإعداد، تنشئ repositories وworkers، ثم تسجل GraphQL وgRPC وWebSocket وتبدأ المستهلكين الخلفيين.

بعد وصول mutation `createUploadSession` إلى GraphQL، يمر الطلب إلى `CreateSessionUseCase.Execute` في `internal/application/upload/create_session.go`. الترتيب الفعلي هو: `ValidateUploadRequest`، ثم Redis rate limit بالمفتاح `upload:<userId>`، ثم idempotency إذا أُرسل المفتاح، ثم قراءة quota وعدد العمليات النشطة، ثم إنشاء `mediaID` و`uploadID`، ثم بناء object key من جهة الخادم، ثم حساب عدد الأجزاء، ثم `CreateMultipartUpload` في S3، ثم إنشاء presigned PUT لكل جزء، ثم حفظ `Media` وحفظ `UploadSession`، ثم الانتقال من `PENDING` إلى `UPLOADING` وزيادة عدّاد الاستخدام.

| خطوة | الملف/الدالة الفعلية | الحكم |
|---|---|---|
| فحص الحجم والنوع والامتداد | `validation.Validator.ValidateUploadRequest` | مطبق قبل الرفع |
| rate limiting | `CreateSessionUseCase.Execute` وshared Redis limiter | مطبق |
| idempotency للإنشاء | `CheckAndStoreIdempotency` | مطبق عند إرسال `IdempotencyKey`، مع ضرورة التأكد أن الـ resolver يمرر المفتاح فعلًا |
| quota وconcurrent uploads | `quotaRepo.GetUsage`, `CanUpload`, `HasUploadSlot` | مطبق، لكن يجب اختبار race تحت التزامن |
| إنشاء IDs وobject key | `uuid.NewString`, `buildObjectKey` | مطبق؛ العميل لا يختار المسار النهائي |
| Multipart S3 | `storage.CreateMultipartUpload` | مطبق |
| Presigned PUT URLs | `storage.GeneratePresignedParts` | مطبق |
| حفظ metadata/session | `mediaRepo.Create`, `uploadRepo.Create` | مطبق |
| زيادة active upload | `quotaRepo.IncrementUsage` | موجودة، لكن الكود يسجل الخطأ ولا يفشل العملية؛ يلزم قرار واضح للـ production |

## B. المسار الفعلي لإكمال الرفع

الدالة `CompleteUploadUseCase.Execute` في `complete_upload.go` تجلب الجلسة، تقارن `session.UserID` مع `in.UserID`، ترفض الجلسة المنتهية، تستخدم lock موزعًا في Redis، ثم تستدعي `CompleteMultipartUpload`. بعد ذلك تستعمل `HeadObject` وتقارن الحجم الفعلي بالحجم المعلن. ثم تستدعي `ValidatePostUpload` لفحص magic bytes وحساب SHA-256 إذا أرسل العميل checksum. عند الفشل تنتقل الحالة إلى `FAILED` ولا ينبغي إصدار تنزيل.

عند النجاح ينشئ الكود outbox event من نوع `media.upload.completed` قبل تغيير الحالة إلى `UPLOADED`، ثم يحدّث UploadSession إلى `COMPLETED`، ويقلل عداد الرفع النشط ويضيف الحجم إلى الاستخدام. هذه نقطة قوية في التصميم، لكن إنشاء outbox مصنف في الكود كخطأ غير قاتل؛ لذلك يلزم اختبار crash consistency ومراجعة ما إذا كانت عملية حفظ outbox وقراءة media تستخدم نفس استراتيجية atomicity المطلوبة.

## C. ما يفعله Frontend فعليًا

`frontend/src/api/client.ts` يضع `Authorization: Bearer <accessToken>` في طلبات Apollo GraphQL، ويستخدم `VITE_GATEWAY_URL` أو `/graphql`. ملف `operations.ts` يعرّف عمليات التسجيل وتسجيل الدخول وإنشاء الجلسة وإكمالها وإلغائها وحذف media والحصول على Download URL والاستعلام عن حالة الرفع.

`frontend/src/upload/advancedUploader.ts` يطبق chunking بحجم `5 MiB`، وconcurrency ثابتًا يساوي `4`، وخمس محاولات، وexponential backoff مع jitter. يحسب SHA-256 قبل إنشاء الجلسة، يحفظ الجلسة في IndexedDB، يرفع كل part باستخدام `XMLHttpRequest` للحصول على progress دقيق، يلتقط `ETag`، يحفظ الأجزاء المكتملة، ثم يرسلها إلى `completeUpload`.

يوجد دعم pause/resume/cancel وحالات offline/online. عند انقطاع الشبكة يوقف العمل الجديد، وعند عودتها يحاول الاستئناف. لكن توجد ملاحظتان مهمتان: أولًا، `presignedParts` المخزنة قد تنتهي صلاحيتها، والكود نفسه يعلّق على الحاجة إلى إعادة طلب URLs؛ لذلك لا نعد بأن resume بعد انتهاء URL مطبق كاملًا حتى يثبت اختبار integration ذلك. ثانيًا، `fileId` المحلي مشتق من الاسم والحجم ووقت التعديل، وهو مناسب للتعرف المحلي لكنه ليس بديلًا عن `mediaId` أو ownership server-side.

`frontend/src/upload/indexedDB.ts` ينشئ database باسم `MediaUploadDB` وstore باسم `sessions` وبـ `fileId` كمفتاح. يحفظ `mediaId`, `uploadId`, `s3UploadId`, `partSize`, `completedParts`, و`presignedParts` ووقت الانتهاء. هذا يحقق persistence محليًا، لكنه لا يجعل IndexedDB مصدر الحقيقة؛ الحقيقة النهائية هي Backend وS3.

## D. WebSocket: ما هو مطبق وما يجب الحذر منه

`frontend/src/upload/websocket.ts` يحدد URL من `VITE_MEDIA_WS_URL`، أو يبنيه من `window.location` ويستخدم `wss` عند HTTPS. يرسل token في query string، يعيد الاتصال بـ exponential backoff وjitter حتى 30 ثانية، ويحوّل الأحداث إلى status. التعليق البرمجي يذكر بوضوح أن WebSocket ليس مصدر الحقيقة، وأن الواجهة ينبغي أن تعيد جلب الحالة من Backend بعد reconnect.

يجب اختبار أن طبقة Go في `internal/transport/ws` تتحقق فعليًا من JWT وتربط subject بقناة المستخدم، لا أن تعتمد فقط على قيمة query. كما يجب عدم اعتبار event مفقود أثناء disconnect فشلًا في الرفع؛ عند reconnect يجب تنفيذ reconciliation عبر `uploadStatus` أو query media.

## E. مصفوفة حالة المتطلبات المطلوبة

| المتطلب | الحالة الأدق | الدليل أو سبب التصنيف |
|---|---|---|
| Authentication & Authorization للرفع والتنزيل والحذف | مطبق في المسار الأساسي | GraphQL يستخرج المستخدم، وuse cases تتحقق من ownership؛ يلزم إكمال اختبار الأدوار وACL المشتركة |
| Upload Session وIDs وmetadata | مطبق | `CreateSessionUseCase` وDynamoDB repositories |
| Validation قبل الرفع | مطبق جزئيًا إلى مطبق | الحجم وMIME والامتداد والـ rate/quota موجودة؛ permission التفصيلية وIP limits تحتاج إثبات integration |
| Presigned URLs ورفع مباشر إلى S3 | مطبق | S3 adapter يعيد presigned parts، والواجهة ترفع PUT مباشرة |
| Multipart وparallel upload | مطبق | backend يحسب parts والواجهة ترفع 4 أجزاء بالتوازي |
| State machine | مطبق جزئيًا | الحالات الأساسية موجودة، لكن يلزم التحقق من كل transitions والـ terminal states واختبارات سباق |
| DynamoDB metadata/ownership/timestamps/TTL | مطبق | repositories وEnsureTTL في `main.go`؛ TTL deletion غير فوري ويحتاج اختبار runtime |
| Redis progress/rate/locks/idempotency | مطبق جزئيًا | rate limiter وlock وidempotency واضحة؛ progress الدائم ليس بديلًا عن DynamoDB |
| Checksum وmagic bytes | مطبق | client SHA-256 اختياري و`ValidatePostUpload` يفحص المحتوى الفعلي |
| Virus scanning وQUARANTINED | worker pipeline موجود | وجود scan worker لا يثبت وحده توفر ClamAV أو أن كل مسار يحظر التنزيل عند الإصابة؛ يلزم اختبار تشغيل فعلي |
| Compression | worker موجود | يلزم إثبات binary/configuration ونتيجة rendition في البيئة المستهدفة |
| Image thumbnails/WebP/AVIF | worker موجود/جزئي | وجود `image/worker.go` لا يكفي لإثبات نجاح كل codec والأحجام؛ يلزم test artifact |
| Video resolutions/HLS/FFmpeg | worker موجود/جزئي | يلزم إثبات playlist/segments وplayback، وليس مجرد وجود FFmpeg code |
| Metadata extraction | موجود | `metadata` worker وextractors موجودان؛ يلزم فحص الحقول المحفوظة واختبار ملفات متنوعة |
| Kafka events وconsumer groups | مطبق | `main.go` يربط upload completed وscan completed وdelete requested بالمستهلكين |
| Worker pool | مطبق | pools منفصلة للفحص والصور والفيديو والحذف والضغط والـ metadata |
| Retry/backoff وDLQ | مطبق جزئيًا | consumers لديها MaxRetries وDLQ؛ يلزم اختبار فشل متعمد وreplay |
| WebSocket realtime | مطبق | NATS publisher وHub وfrontend reconnect موجودة |
| CDN | غير مثبت/مستبعد في هذا الإصدار | وجود presigned S3 لا يساوي وجود CloudFront/CDN configuration |
| Delete asynchronous | مطبق | delete use case ينشئ outbox والـ delete worker ينفذ الخلفية |
| Resume upload | مطبق جزئيًا | IndexedDB وstatus وETags موجودة؛ إعادة توقيع URLs بعد انتهاء الصلاحية تحتاج endpoint/test |
| Cancel upload | مطبق | abort mutation وAbortMultipartUpload والمسار إلى `ABORTED` موجود |
| Quota race protection | موجود جزئيًا | الفحوص موجودة، لكن reservation الذرية تحت concurrent create يجب إثباتها باختبار ضغط |
| Conditional writes | مطبق جزئيًا إلى مطبق | repositories تستخدم transitions مشروطة في مسارات مهمة؛ يجب مراجعة كل repository لاكتشاف أي update غير مشروط |
| Reconciliation cron | مطبق | reconciliation worker يبدأ من `main.go` ويعالج الحالات العالقة؛ يلزم اختبار stale records |
| S3 lifecycle incomplete uploads | مفعّل برمجيًا | `EnsureBucketLifecycle` يستدعى عند بدء الخدمة، لكن يلزم قراءة configuration من S3/LocalStack وإثباتها |
| DynamoDB TTL | مفعّل برمجيًا | `EnsureTTL` يستدعى، لكن cleanup asynchronous ويحتاج اختبارًا مستقلًا |
| Logging/metrics/tracing | موجود | shared logging، Prometheus metrics، OpenTelemetry initialization؛ يلزم التأكد من وصول series والـ spans فعليًا |
| Google Drive | غير مطبق في هذا المستودع | لا توجد OAuth handlers أو connector أو import/export worker ظاهر في media pipeline |
| YouTube | غير مطبق | لا توجد publish workflow أو YouTube API integration ظاهرة |
| Dropbox | غير مطبق | لا توجد OAuth/API adapter أو sync state ظاهرة |

## F. خطوات تشغيل واختبار الرفع من البداية للنهاية

1. شغّل البنية التحتية المحلية كما يحددها `docker-compose.infra.yml` أو ملفات Kubernetes/Skaffold. تحقق من أن Redis وKafka وNATS وDynamoDB/LocalStack وS3/LocalStack جاهزة، ولا تكتفِ بأن الحاويات في حالة `Running`.

2. شغّل `media-service` بعد ضبط environment variables الخاصة بـ AWS region وbucket وDynamoDB table وRedis وKafka وNATS وports. راقب logs الخاصة بإنشاء table وbucket وCORS وTTL وLifecycle.

3. شغّل الـ gateway والـ frontend، ثم سجّل الدخول. تأكد في DevTools أن GraphQL requests تحتوي على Bearer token، وأنه لا يوجد AWS access key أو secret في bundle أو localStorage.

4. اختر ملفًا صغيرًا صالحًا. يجب أن ينفذ frontend client validation، ثم يحسب checksum، ثم يرسل `createUploadSession`. سجّل `mediaId`, `uploadId`, `s3UploadId`, `partSize`, `totalParts`, ووقت انتهاء URLs.

5. راقب Network tab: طلبات parts يجب أن تتجه إلى S3/LocalStack presigned URLs لا إلى Gateway. تحقق أن كل PUT يعيد status نجاح و`ETag`، وأن عدد الطلبات المتوازية لا يتجاوز الحد المقصود.

6. افصل الشبكة أثناء part محدد ثم أعدها. يجب أن يظل الجزء المكتمل محفوظًا، وألا تعاد الأجزاء الناجحة، وأن يعمل retry للجزء الفاشل فقط. إذا انتهت presigned URLs أثناء التوقف، يجب أن يظهر بوضوح أن إعادة التوقيع غير متاحة أو أن endpoint التجديد نفّذها.

7. بعد اكتمال parts، راقب mutation `completeUpload`. يجب أن ينفذ Backend `HeadObject` ثم size check ثم magic-byte/checksum validation ثم outbox event. افحص DynamoDB وS3 وKafka بدل الاعتماد على progress UI فقط.

8. راقب انتقالات الحالة: `UPLOADING` ثم `UPLOADED` ثم `SCANNING` ثم `PROCESSING` ثم `READY`. وجود `DONE` في الواجهة يعني أن complete request انتهى، وليس بالضرورة أن الملف أصبح `READY` بعد scan والمعالجة.

9. اختبر ملفًا امتداده صحيح لكن magic bytes خاطئة، وملفًا أكبر من الحد، وMIME غير مسموح، وchecksum خاطئ، وquota ممتلئة. يجب أن ترفض الطبقة المناسبة الطلب، ولا تمنح Download URL لملف فشل أو quarantine.

10. اختبر التنزيل عبر `downloadUrl`، ثم اختبر الحذف من GraphQL فقط. لا تستخدم credentials في المتصفح ولا تحذف object مباشرة من S3. راقب `DELETE_REQUESTED` والـ worker ثم `DELETE_COMPLETED` أو `DELETE_FAILED`.

11. نفّذ اختبارات التكرار: أرسل create مرتين بنفس idempotency key، وأرسل complete مرتين، وأرسل delete مرتين، وشغّل consumer event نفسه مرتين. النتيجة الصحيحة هي side effect واحد وحالة نهائية سليمة.

## G. طريقة إضافة Google Drive أو YouTube أو Dropbox لاحقًا

لا ينبغي إدخال هذه الخدمات داخل `advancedUploader.ts` ولا إعطاء browser مفاتيح طويلة العمر. التصميم المقترح هو إضافة integration module مستقل يحتوي OAuth callback وtoken vault وprovider adapter وjob record وworker. عند import، ينشئ backend upload session أو ينسخ stream إلى object مؤقت ثم يمرره بنفس validation/scan pipeline. عند export، لا يقرأ provider مباشرة من الواجهة؛ بل ينشئ job مصرحًا، ويقرأ rendition جاهزة من media-service، ثم يرفعها إلى provider.

ينبغي إضافة حقول مثل `externalProvider`, `externalObjectId`, `externalVersion`, `syncState`, `lastSyncedAt` و`permissionSnapshot` إلى نموذج integration، مع idempotency key لكل import/export. تظل صلاحية الملف الداخلي في `media-service`، ولا تتحول صلاحيات Google Drive أو Dropbox أو YouTube تلقائيًا إلى ACL داخلية من دون سياسة مزامنة صريحة.

## H. خلاصة عملية للمطور الذي لا يعرف المشروع

إذا أردت فهم المشروع بسرعة، ابدأ من `frontend/src/upload/advancedUploader.ts` لتعرف ما يرسله المتصفح، ثم انتقل إلى `frontend/src/api/operations.ts` لمعرفة GraphQL contract، ثم إلى `media-service/internal/transport/graphql/handler.go` لمعرفة resolver mapping، ثم إلى `create_session.go` و`complete_upload.go` لمعرفة business flow الحقيقي، ثم إلى `internal/adapters/s3` و`dynamodb` لمعرفة التخزين، وأخيرًا إلى `workers` و`adapters/kafka` و`transport/ws` لمعرفة المعالجة غير المتزامنة والتحديثات اللحظية.

القاعدة الأهم هي أن **الواجهة تعرض الحالة ولا تقرر الصلاحية**، وأن **S3 يحمل bytes ولا يملك business state**، وأن **DynamoDB يحفظ ownership والحالة**، وأن **Kafka يحمل workflow events**، وأن **WebSocket يسرّع عرض النتيجة لكنه ليس مصدر الحقيقة**. أما Google Drive وYouTube وDropbox فهي تكاملات مستقبلية منفصلة وليست جزءًا مثبتًا من pipeline الحالي.

## المراجع الخارجية

[1]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/mpuoverview.html "Amazon S3 Multipart Upload Overview"
[2]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/PresignedUrlUploadObject.html "Uploading objects with presigned URLs"
[3]: https://docs.aws.amazon.com/amazons3/latest/userguide/object-lifecycle-mgmt.html "Managing your storage lifecycle"
[4]: https://docs.aws.amazon.com/amazons3/latest/userguide/using-presigned-url.html "Using presigned URLs"
[5]: https://docs.aws.amazon.com/amazons3/latest/userguide/RangeGET.html "Getting objects using byte-range fetches"
[6]: https://docs.aws.amazon.com/amazons3/latest/userguide/checking-object-integrity.html "Checking object integrity"

> هذه المراجع تشرح سلوك S3 العام فقط. الحكم على ما هو مطبق في المشروع مبني على ملفات المستودع الفعلية المذكورة داخل هذا الدليل، وليس على الوثائق الخارجية.
