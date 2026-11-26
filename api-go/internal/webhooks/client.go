package webhooks

// Message is one request to a webhook. Body is sent byte for byte and signed
// with Secret.
type Message struct {
	URL       string
	Secret    string
	EventID   string
	EventType EventType
	Body      []byte
}
