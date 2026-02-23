package chat

import "encoding/json"

// Message represents a Minecraft JSON chat message.
type Message struct {
	Text          string    `json:"text"`
	Bold          bool      `json:"bold,omitempty"`
	Italic        bool      `json:"italic,omitempty"`
	Underlined    bool      `json:"underlined,omitempty"`
	Strikethrough bool      `json:"strikethrough,omitempty"`
	Obfuscated    bool      `json:"obfuscated,omitempty"`
	Color         string    `json:"color,omitempty"`
	Extra         []Message `json:"extra,omitempty"`
}

// String serializes the message to JSON.
func (m Message) String() string {
	b, _ := json.Marshal(m)
	return string(b)
}

// Text creates a simple text message.
func Text(text string) Message {
	return Message{Text: text}
}

// Colored creates a colored text message.
func Colored(text, color string) Message {
	return Message{Text: text, Color: color}
}

// WithExtra creates a message with additional components appended.
func WithExtra(text string, extra ...Message) Message {
	msg := Message{Text: text}
	if len(extra) > 0 {
		msg.Extra = extra
	}
	return msg
}
