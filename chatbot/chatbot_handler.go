package chatbot

import (
	"context"
	"encoding/json"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 18:30
 */

type IMessageHandler func(c context.Context, data *BotCallbackDataModel) error

type DefaultChatBotFrameHandler struct {
	defaultHandler IMessageHandler
}

func NewDefaultChatBotFrameHandler(defaultHandler IMessageHandler) *DefaultChatBotFrameHandler {
	return &DefaultChatBotFrameHandler{
		defaultHandler: defaultHandler,
	}
}

func (h *DefaultChatBotFrameHandler) OnEventReceived(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	frameResp := &payload.DataFrameResponse{
		Code: 200,
		Headers: payload.DataFrameHeader{
			payload.DataFrameHeaderKContentType: payload.DataFrameContentTypeKJson,
			payload.DataFrameHeaderKMessageId:   df.GetMessageId(),
		},
		Message: "ok",
		Data:    "",
	}

	msgData := &BotCallbackDataModel{}
	err := json.Unmarshal([]byte(df.Data), msgData)
	if err != nil {
		return nil, err
	}

	if h.defaultHandler != nil {
		err = h.defaultHandler(ctx, msgData)
		if err != nil {
			return nil, err
		}
	}

	return frameResp, nil
}
