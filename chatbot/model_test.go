package chatbot

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBotCallbackDataModel_UnmarshalRobotCode(t *testing.T) {
	raw := `{
		"conversationId":"cidxxx",
		"msgId":"msgxxx",
		"senderNick":"Alice",
		"msgtype":"text",
		"robotCode":"ding0f4lethz0fonxvz6",
		"text":{"content":"hello"}
	}`

	var model BotCallbackDataModel
	require.NoError(t, json.Unmarshal([]byte(raw), &model))
	assert.Equal(t, "ding0f4lethz0fonxvz6", model.RobotCode)
	assert.Equal(t, "hello", model.Text.Content)
	assert.Equal(t, "text", model.Msgtype)
}
