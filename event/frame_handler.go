package event

import (
	"context"
	"encoding/json"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

/**
 * @Author linya.jj
 * @Date 2023/4/26 17:15
 */

type DefaultEventFrameHandler struct {
	defaultHandler IEventHandler
}

func NewDefaultEventFrameHandler(defaultHandler IEventHandler) *DefaultEventFrameHandler {
	return &DefaultEventFrameHandler{
		defaultHandler: defaultHandler,
	}
}

func (h *DefaultEventFrameHandler) OnEventReceived(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	eventHeader := NewEventHeaderFromDataFrame(df)

	if h.defaultHandler == nil {
		logger.GetLogger().Warningf("No event handler found, drop this event. eventType=[%s], eventId=[%s], eventCorpId=[%s]",
			eventHeader.EventType, eventHeader.EventId, eventHeader.EventCorpId)

		return nil, nil
	}

	ret, err := h.defaultHandler(ctx, eventHeader, []byte(df.Data))
	if err != nil {
		logger.GetLogger().Errorf("Event handler process error. eventType=[%s], eventId=[%s], eventCorpId=[%s] err=[%s]",
			eventHeader.EventType, eventHeader.EventId, eventHeader.EventCorpId, err)

		ret = EventProcessStatusKLater
	}

	result := NewEventProcessResultSuccess()
	code := payload.DataFrameResponseStatusCodeKOK
	if ret != EventProcessStatusKSuccess {
		code = payload.DataFrameResponseStatusCodeKInternalError
		result = NewEventProcessResultLater()
	}

	resultStr, _ := json.Marshal(result)

	frameResp := &payload.DataFrameResponse{
		Code: code,
		Headers: payload.DataFrameHeader{
			payload.DataFrameHeaderKContentType: payload.DataFrameContentTypeKJson,
			payload.DataFrameHeaderKMessageId:   df.GetMessageId(),
		},
		Message: "ok",
		Data:    string(resultStr),
	}

	return frameResp, nil
}
