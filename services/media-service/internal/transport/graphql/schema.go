package graphql

import (
	"fmt"

	gql "github.com/graphql-go/graphql"
)

const subgraphSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: [])

type MediaVersion {
  versionType: String!
  objectKey: String!
  contentType: String!
  size: Float!
  width: Int
  height: Int
  durationMs: Float
}

type Media {
  mediaId: String!
  ownerId: String!
  fileName: String!
  contentType: String!
  mediaType: String!
  size: Float!
  status: String!
  objectKey: String!
  createdAt: Float!
  updatedAt: Float!
  versions: [MediaVersion]
}

type PresignedPart {
  partNumber: Int!
  presignedUrl: String!
}

type UploadSession {
  mediaId: String!
  uploadId: String!
  s3UploadId: String!
  presignedParts: [PresignedPart]
  partSize: Float!
  totalParts: Int!
  expiresAt: Float!
}

type UploadStatus {
  uploadId: String!
  status: String!
  totalParts: Int!
  completedParts: Int!
  missingParts: [Int]
  expiresAt: Float!
}

type CompleteUploadResult {
  mediaId: String!
  status: String!
}

type PaginationInfo {
  totalItems: Int!
  currentPage: Int!
  nextPage: Int
}

type MediaListData {
  paginationInfo: PaginationInfo!
  items: [Media!]!
}

type DownloadUrl {
  url: String!
  expiresAt: Float!
  contentType: String!
}

type Quota {
  usedBytes: Float!
  quotaBytes: Float!
  remainingBytes: Float!
  activeUploads: Int!
  maxConcurrentUploads: Int!
}

# DLQ types
type DLQMessage {
  id: String!
  topic: String!
  partition: Int!
  offset: Float!
  key: String!
  value: String!
  headers: [String]
  error: String
  retryCount: Int!
  createdAt: Float!
  originalTimestamp: Float!
}

type DLQStats {
  topic: String!
  messageCount: Int!
}

type DLQReplayResult {
  success: Boolean!
  replayedCount: Int!
  errors: [String]
}

type RenewPresignedResult {
  uploadId: String!
  s3UploadId: String!
  presignedParts: [PresignedPart]!
  partSize: Float!
  totalParts: Int!
  expiresAt: Float!
}

# Standard Envelope Responses
type MediaResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: Media
}

type MediaListResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: MediaListData
}

type UploadSessionResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: UploadSession
}

type UploadStatusResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: UploadStatus
}

type CompleteUploadResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: CompleteUploadResult
}

type DownloadUrlResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: DownloadUrl
}

type QuotaResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: Quota
}

type BaseResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: String
}

type DLQStatsResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: [DLQStats!]
}

type DLQReplayResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: DLQReplayResult
}

type RenewPresignedResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: RenewPresignedResult
}

input CreateUploadSessionInput {
  fileName: String!
  contentType: String!
  size: Float!
  checksum: String
  deliveryId: String
  idempotencyKey: String
}

input RenewPresignedInput {
  uploadId: String!
  expirySeconds: Int
}

input UploadPartInput {
  partNumber: Int!
  eTag: String!
}

input CompleteUploadInput {
  uploadId: String!
  parts: [UploadPartInput!]!
  idempotencyKey: String
}

type Query {
  _service: _Service!
  media(mediaId: String!): MediaResponse!
  listMedia(ownerId: String!, limit: Int, cursor: String, statusFilter: String): MediaListResponse!
  uploadStatus(uploadId: String!): UploadStatusResponse!
  downloadUrl(mediaId: String!, versionType: String, expirySeconds: Int, range: String): DownloadUrlResponse!
  quota: QuotaResponse!
  dlqTopics: [String]!
  dlqStats(topics: [String]!): DLQStatsResponse!
}

type Mutation {
  createUploadSession(input: CreateUploadSessionInput!): UploadSessionResponse!
  completeUpload(input: CompleteUploadInput!): CompleteUploadResponse!
  abortUpload(uploadId: String!): BaseResponse!
  deleteMedia(mediaId: String!, idempotencyKey: String): BaseResponse!
  dlqReplay(topic: String!, maxMessages: Int): DLQReplayResponse!
  renewPresigned(input: RenewPresignedInput!): RenewPresignedResponse!
}

type _Service {
  sdl: String!
}`

var paginationInfoType = gql.NewObject(gql.ObjectConfig{
	Name: "PaginationInfo",
	Fields: gql.Fields{
		"totalItems":  &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"currentPage": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"nextPage":    &gql.Field{Type: gql.Int},
	},
})

var mediaVersionType = gql.NewObject(gql.ObjectConfig{
	Name: "MediaVersion",
	Fields: gql.Fields{
		"versionType": &gql.Field{Type: gql.NewNonNull(gql.String)},
		"objectKey":   &gql.Field{Type: gql.NewNonNull(gql.String)},
		"contentType": &gql.Field{Type: gql.NewNonNull(gql.String)},
		"size":        &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"width":       &gql.Field{Type: gql.Int},
		"height":      &gql.Field{Type: gql.Int},
		"durationMs":  &gql.Field{Type: gql.Float},
	},
})

var mediaType = gql.NewObject(gql.ObjectConfig{
	Name: "Media",
	Fields: gql.Fields{
		"mediaId":     &gql.Field{Type: gql.NewNonNull(gql.String)},
		"ownerId":     &gql.Field{Type: gql.NewNonNull(gql.String)},
		"fileName":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"contentType": &gql.Field{Type: gql.NewNonNull(gql.String)},
		"mediaType":   &gql.Field{Type: gql.NewNonNull(gql.String)},
		"size":        &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"status":      &gql.Field{Type: gql.NewNonNull(gql.String)},
		"objectKey":   &gql.Field{Type: gql.NewNonNull(gql.String)},
		"createdAt":   &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"updatedAt":   &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"versions":    &gql.Field{Type: gql.NewList(mediaVersionType)},
	},
})

var presignedPartType = gql.NewObject(gql.ObjectConfig{
	Name: "PresignedPart",
	Fields: gql.Fields{
		"partNumber":   &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"presignedUrl": &gql.Field{Type: gql.NewNonNull(gql.String)},
	},
})

var uploadSessionType = gql.NewObject(gql.ObjectConfig{
	Name: "UploadSession",
	Fields: gql.Fields{
		"mediaId":        &gql.Field{Type: gql.NewNonNull(gql.String)},
		"uploadId":       &gql.Field{Type: gql.NewNonNull(gql.String)},
		"s3UploadId":     &gql.Field{Type: gql.NewNonNull(gql.String)},
		"presignedParts": &gql.Field{Type: gql.NewList(presignedPartType)},
		"partSize":       &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"totalParts":     &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"expiresAt":      &gql.Field{Type: gql.NewNonNull(gql.Float)},
	},
})

var uploadStatusType = gql.NewObject(gql.ObjectConfig{
	Name: "UploadStatus",
	Fields: gql.Fields{
		"uploadId":       &gql.Field{Type: gql.NewNonNull(gql.String)},
		"status":         &gql.Field{Type: gql.NewNonNull(gql.String)},
		"totalParts":     &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"completedParts": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"missingParts":   &gql.Field{Type: gql.NewList(gql.Int)},
		"expiresAt":      &gql.Field{Type: gql.NewNonNull(gql.Float)},
	},
})

var completeUploadResultType = gql.NewObject(gql.ObjectConfig{
	Name: "CompleteUploadResult",
	Fields: gql.Fields{
		"mediaId": &gql.Field{Type: gql.NewNonNull(gql.String)},
		"status":  &gql.Field{Type: gql.NewNonNull(gql.String)},
	},
})

var mediaListDataType = gql.NewObject(gql.ObjectConfig{
	Name: "MediaListData",
	Fields: gql.Fields{
		"paginationInfo": &gql.Field{Type: gql.NewNonNull(paginationInfoType)},
		"items":          &gql.Field{Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(mediaType)))},
	},
})

var downloadUrlType = gql.NewObject(gql.ObjectConfig{
	Name: "DownloadUrl",
	Fields: gql.Fields{
		"url":         &gql.Field{Type: gql.NewNonNull(gql.String)},
		"expiresAt":   &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"contentType": &gql.Field{Type: gql.NewNonNull(gql.String)},
	},
})

var quotaType = gql.NewObject(gql.ObjectConfig{
	Name: "Quota",
	Fields: gql.Fields{
		"usedBytes":            &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"quotaBytes":           &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"remainingBytes":       &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"activeUploads":        &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"maxConcurrentUploads": &gql.Field{Type: gql.NewNonNull(gql.Int)},
	},
})

var dlqStatsType = gql.NewObject(gql.ObjectConfig{
	Name: "DLQStats",
	Fields: gql.Fields{
		"topic":        &gql.Field{Type: gql.NewNonNull(gql.String)},
		"messageCount": &gql.Field{Type: gql.NewNonNull(gql.Int)},
	},
})

var dlqReplayResultType = gql.NewObject(gql.ObjectConfig{
	Name: "DLQReplayResult",
	Fields: gql.Fields{
		"success":       &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"replayedCount": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"errors":        &gql.Field{Type: gql.NewList(gql.String)},
	},
})

var renewPresignedResultType = gql.NewObject(gql.ObjectConfig{
	Name: "RenewPresignedResult",
	Fields: gql.Fields{
		"uploadId":       &gql.Field{Type: gql.NewNonNull(gql.String)},
		"s3UploadId":     &gql.Field{Type: gql.NewNonNull(gql.String)},
		"presignedParts": &gql.Field{Type: gql.NewNonNull(gql.NewList(presignedPartType))},
		"partSize":       &gql.Field{Type: gql.NewNonNull(gql.Float)},
		"totalParts":     &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"expiresAt":      &gql.Field{Type: gql.NewNonNull(gql.Float)},
	},
})

// Wrapper Object Types
var mediaResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "MediaResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: mediaType},
	},
})

var mediaListResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "MediaListResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: mediaListDataType},
	},
})

var uploadSessionResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "UploadSessionResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: uploadSessionType},
	},
})

var uploadStatusResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "UploadStatusResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: uploadStatusType},
	},
})

var completeUploadResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "CompleteUploadResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: completeUploadResultType},
	},
})

var downloadUrlResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "DownloadUrlResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: downloadUrlType},
	},
})

var quotaResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "QuotaResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: quotaType},
	},
})

var baseResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "BaseResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: gql.String},
	},
})

var dlqStatsResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "DLQStatsResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: gql.NewList(gql.NewNonNull(dlqStatsType))},
	},
})

var dlqReplayResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "DLQReplayResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: dlqReplayResultType},
	},
})

var renewPresignedResponseType = gql.NewObject(gql.ObjectConfig{
	Name: "RenewPresignedResponse",
	Fields: gql.Fields{
		"success":    &gql.Field{Type: gql.NewNonNull(gql.Boolean)},
		"statusCode": &gql.Field{Type: gql.NewNonNull(gql.Int)},
		"message":    &gql.Field{Type: gql.NewNonNull(gql.String)},
		"timeStamp":  &gql.Field{Type: gql.NewNonNull(gql.String)},
		"data":       &gql.Field{Type: renewPresignedResultType},
	},
})

var serviceType = gql.NewObject(gql.ObjectConfig{
	Name: "_Service",
	Fields: gql.Fields{
		"sdl": &gql.Field{Type: gql.NewNonNull(gql.String)},
	},
})

var createUploadSessionInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "CreateUploadSessionInput",
	Fields: gql.InputObjectConfigFieldMap{
		"fileName":       &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		"contentType":    &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		"size":           &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.Float)},
		"checksum":       &gql.InputObjectFieldConfig{Type: gql.String},
		"deliveryId":     &gql.InputObjectFieldConfig{Type: gql.String},
		"idempotencyKey": &gql.InputObjectFieldConfig{Type: gql.String},
	},
})

var uploadPartInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "UploadPartInput",
	Fields: gql.InputObjectConfigFieldMap{
		"partNumber": &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.Int)},
		"eTag":       &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
	},
})

var completeUploadInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "CompleteUploadInput",
	Fields: gql.InputObjectConfigFieldMap{
		"uploadId": &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		"parts": &gql.InputObjectFieldConfig{
			Type: gql.NewNonNull(gql.NewList(gql.NewNonNull(uploadPartInput))),
		},
		"idempotencyKey": &gql.InputObjectFieldConfig{Type: gql.String},
	},
})

var renewPresignedInput = gql.NewInputObject(gql.InputObjectConfig{
	Name: "RenewPresignedInput",
	Fields: gql.InputObjectConfigFieldMap{
		"uploadId":      &gql.InputObjectFieldConfig{Type: gql.NewNonNull(gql.String)},
		"expirySeconds": &gql.InputObjectFieldConfig{Type: gql.Int},
	},
})

func buildSchema(h *Handler) (gql.Schema, error) {
	query := gql.NewObject(gql.ObjectConfig{
		Name: "Query",
		Fields: gql.Fields{
			"_service": &gql.Field{
				Type: gql.NewNonNull(serviceType),
				Resolve: func(p gql.ResolveParams) (interface{}, error) {
					return map[string]interface{}{"sdl": subgraphSDL}, nil
				},
			},
			"media": &gql.Field{
				Type: gql.NewNonNull(mediaResponseType),
				Args: gql.FieldConfigArgument{
					"mediaId": &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
				},
				Resolve: h.resolveMedia,
			},
			"listMedia": &gql.Field{
				Type: gql.NewNonNull(mediaListResponseType),
				Args: gql.FieldConfigArgument{
					"ownerId":      &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
					"limit":        &gql.ArgumentConfig{Type: gql.Int},
					"cursor":       &gql.ArgumentConfig{Type: gql.String},
					"statusFilter": &gql.ArgumentConfig{Type: gql.String},
				},
				Resolve: h.resolveListMedia,
			},
			"uploadStatus": &gql.Field{
				Type: gql.NewNonNull(uploadStatusResponseType),
				Args: gql.FieldConfigArgument{
					"uploadId": &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
				},
				Resolve: h.resolveUploadStatus,
			},
			"downloadUrl": &gql.Field{
				Type: gql.NewNonNull(downloadUrlResponseType),
				Args: gql.FieldConfigArgument{
					"mediaId":       &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
					"versionType":   &gql.ArgumentConfig{Type: gql.String},
					"expirySeconds": &gql.ArgumentConfig{Type: gql.Int},
					"range":         &gql.ArgumentConfig{Type: gql.String},
				},
				Resolve: h.resolveDownloadUrl,
			},
			"quota": &gql.Field{
				Type:    gql.NewNonNull(quotaResponseType),
				Resolve: h.resolveQuota,
			},
			"dlqTopics": &gql.Field{
				Type:    gql.NewNonNull(gql.NewList(gql.NewNonNull(gql.String))),
				Resolve: h.resolveDLQTopics,
			},
			"dlqStats": &gql.Field{
				Type: gql.NewNonNull(dlqStatsResponseType),
				Args: gql.FieldConfigArgument{
					"topics": &gql.ArgumentConfig{Type: gql.NewList(gql.String)},
				},
				Resolve: h.resolveDLQStats,
			},
		},
	})

	mutation := gql.NewObject(gql.ObjectConfig{
		Name: "Mutation",
		Fields: gql.Fields{
			"createUploadSession": &gql.Field{
				Type: gql.NewNonNull(uploadSessionResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(createUploadSessionInput)},
				},
				Resolve: h.resolveCreateUploadSession,
			},
			"completeUpload": &gql.Field{
				Type: gql.NewNonNull(completeUploadResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(completeUploadInput)},
				},
				Resolve: h.resolveCompleteUpload,
			},
			"abortUpload": &gql.Field{
				Type: gql.NewNonNull(baseResponseType),
				Args: gql.FieldConfigArgument{
					"uploadId": &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
				},
				Resolve: h.resolveAbortUpload,
			},
			"deleteMedia": &gql.Field{
				Type: gql.NewNonNull(baseResponseType),
				Args: gql.FieldConfigArgument{
					"mediaId":        &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
					"idempotencyKey": &gql.ArgumentConfig{Type: gql.String},
				},
				Resolve: h.resolveDeleteMedia,
			},
			"dlqReplay": &gql.Field{
				Type: gql.NewNonNull(dlqReplayResponseType),
				Args: gql.FieldConfigArgument{
					"topic":       &gql.ArgumentConfig{Type: gql.NewNonNull(gql.String)},
					"maxMessages": &gql.ArgumentConfig{Type: gql.Int},
				},
				Resolve: h.resolveDLQReplay,
			},
			"renewPresigned": &gql.Field{
				Type: gql.NewNonNull(renewPresignedResponseType),
				Args: gql.FieldConfigArgument{
					"input": &gql.ArgumentConfig{Type: gql.NewNonNull(renewPresignedInput)},
				},
				Resolve: h.resolveRenewPresigned,
			},
		},
	})

	schema, err := gql.NewSchema(gql.SchemaConfig{Query: query, Mutation: mutation})
	if err != nil {
		return gql.Schema{}, fmt.Errorf("new schema: %w", err)
	}
	return schema, nil
}