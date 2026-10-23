package handlers

import (
	"bytes"
	"encoding/json"
)

func unmarshalPayload(data []byte) Payload {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		panic(err)
	}
	return Payload{Map: m, Raw: data}
}