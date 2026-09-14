package workers

import (
	"encoding/json"

	pkgevents "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/events"
)

func parseEnvelopeKey(payload []byte, out *pkgevents.EventEnvelope) error {
	return json.Unmarshal(payload, out)
}
