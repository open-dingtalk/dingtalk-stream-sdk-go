package clientV2

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

type EventAckStatus int

type EventHeader string

const (
	SUCCESS EventAckStatus = 1
	LAGER   EventAckStatus = 2

	EVENT_ID   string = "eventId"
	EVENT_TIME string = "eventBornTime"
	CORP_ID    string = "eventCorpId"
	APP_ID     string = "eventUnifiedAppId"
	EVENT_TYPE string = "eventType"
)

type GenericOpenDingTalkEvent struct {
	EventId           string         `json:"eventId"`
	EventBornTime     string         `json:"eventBornTime"`
	EventCorpId       string         `json:"eventCorpId"`
	EventType         string         `json:"eventType"`
	EventUnifiedAppId string         `json:"eventUnifiedAppId"`
	Data              map[string]any `json:"data"`
}

type AckPayload struct {
	Status EventAckStatus `json:"status"`

	Message string `json:"message"`
}

type GenericEventHandler func(event *GenericOpenDingTalkEvent) EventAckStatus

func EventFacade(handler GenericEventHandler) handler.IFrameHandler {
	return func(c context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
		if df == nil {
			return nil, errors.New("empty data frame")
		}
		event := &GenericOpenDingTalkEvent{}
		event.EventId = df.GetHeader(EVENT_ID)
		event.EventBornTime = df.GetHeader(EVENT_TIME)
		event.EventUnifiedAppId = df.GetHeader(APP_ID)
		event.EventType = df.GetHeader(EVENT_TYPE)
		event.EventCorpId = df.GetHeader(CORP_ID)
		data := make(map[string]any, 1)
		if e := json.Unmarshal([]byte(df.Data), &data); e != nil {
			return nil, e
		}
		event.Data = data
		status := safeInvokeEventHandler(event, handler)
		ack := &AckPayload{Status: status}
		response := payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKOK)
		if e := response.SetJson(ack); e != nil {
			return nil, e
		}
		return response, nil
	}
}

func safeInvokeEventHandler(event *GenericOpenDingTalkEvent, handler GenericEventHandler) (status EventAckStatus) {
	defer func() {
		if e := recover(); e != nil {
			logger.GetLogger().Errorf("failed to invoke event handler, error=[%s]", e)
			status = LAGER
		}
	}()
	return handler(event)
}
