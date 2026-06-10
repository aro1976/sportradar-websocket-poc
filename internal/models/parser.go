package models

import "encoding/json"

// ParsePushMessage parses a raw JSON byte slice into a PushMessage.
func ParsePushMessage(data []byte) (PushMessage, error) {
	var msg PushMessage
	err := json.Unmarshal(data, &msg)
	return msg, err
}
