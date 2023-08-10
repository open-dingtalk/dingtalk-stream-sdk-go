package plugin

import (
	"context"
	"encoding/json"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

type CallbackResponse struct {
	Response interface{} `json:"response"`
}

type IPluginMessageHandler func(c context.Context, data *PluginMessage) (interface{}, error)

type DefaultPluginFrameHandler struct {
	defaultHandler IPluginMessageHandler
}

func NewDefaultPluginFrameHandler(defaultHandler IPluginMessageHandler) *DefaultPluginFrameHandler {
	return &DefaultPluginFrameHandler{
		defaultHandler: defaultHandler,
	}
}

func (h *DefaultPluginFrameHandler) OnEventReceived(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	msgData := &PluginMessage{}
	err := json.Unmarshal([]byte(df.Data), msgData)
	if err != nil {
		return nil, err
	}

	if h.defaultHandler == nil {
		return payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKHandlerNotFound), nil
	}

	result, err := h.defaultHandler(ctx, msgData)
	if err != nil {
		return nil, err
	}
	pluginResponse := &PluginResponse{RequestId: msgData.RequestId, Result: result}
	callbackResponse := &CallbackResponse{Response: pluginResponse}
	frameResp := payload.NewSuccessDataFrameResponse()
	frameResp.SetJson(callbackResponse)
	return frameResp, nil
}
