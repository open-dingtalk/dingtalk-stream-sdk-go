package plugin

import (
	"context"
	"encoding/json"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

type CallbackResponse struct {
	Response interface{} `json:"response"`
}

type IDingTalkPluginHandler func(c context.Context, data *DingTalkPluginMessage) (interface{}, error)

type DefaultDingTalkPluginFrameHandler struct {
	defaultHandler IDingTalkPluginHandler
}

func NewDefaultDingTalkPluginFrameHandler(defaultHandler IDingTalkPluginHandler) *DefaultDingTalkPluginFrameHandler {
	return &DefaultDingTalkPluginFrameHandler{
		defaultHandler: defaultHandler,
	}
}

func (h *DefaultDingTalkPluginFrameHandler) OnEventReceived(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	msgData := &DingTalkPluginMessage{}
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
	dingTalkPluginResponse := &DingTalkPluginResponse{RequestId: msgData.RequestId, Result: result}
	callbackResponse := &CallbackResponse{Response: dingTalkPluginResponse}
	frameResp := payload.NewSuccessDataFrameResponse()
	frameResp.SetJson(callbackResponse)
	return frameResp, nil
}
