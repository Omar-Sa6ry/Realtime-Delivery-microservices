package websocket

// WS defaults — mirror the TypeScript WS_DEFAULTS constant.
const (
	// WSDefaultMaxPayload is the maximum accepted message payload in bytes.
	WSDefaultMaxPayload = 16384
	// WSDefaultHeartbeatIntervalMS is the heartbeat send interval.
	WSDefaultHeartbeatIntervalMS = 30000
	// WSDefaultHeartbeatTimeoutMS is the timeout before a silent client is dropped.
	WSDefaultHeartbeatTimeoutMS = 60000
	// WSDefaultConnectionTTLSeconds is the maximum connection lifetime.
	WSDefaultConnectionTTLSeconds = 60
	// WSDefaultPresenceTTLSeconds is the presence entry TTL.
	WSDefaultPresenceTTLSeconds = 90
	// WSDefaultMaxBacklog is the maximum per-connection outbound backlog.
	WSDefaultMaxBacklog = 200
	// WSDefaultSlowConsumerThresholdMS is the threshold after which a consumer is considered slow.
	WSDefaultSlowConsumerThresholdMS = 1000
	// WSDefaultPageSize is the default page size for paginated WS queries.
	WSDefaultPageSize = 50
)