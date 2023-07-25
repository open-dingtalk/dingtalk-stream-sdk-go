package clientV2

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

type EventStatus string

type EventHeader string

const (
	EventStatusSuccess EventStatus = "SUCCESS"
	EventStatusLater   EventStatus = "LATER"

	EventId   string = "eventId"
	EventTime string = "eventBornTime"
	CorpId    string = "eventCorpId"
	AppId     string = "eventUnifiedAppId"
	EventType string = "eventType"
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
	Status EventStatus `json:"status"`

	Message string `json:"message"`
}

type GenericEventHandler func(event *GenericOpenDingTalkEvent) EventStatus

func EventFacade(handler GenericEventHandler) handler.IFrameHandler {
	return func(c context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
		if df == nil {
			return nil, errors.New("empty data frame")
		}
		event := &GenericOpenDingTalkEvent{}
		event.EventId = df.GetHeader(EventId)
		event.EventBornTime = df.GetHeader(EventTime)
		event.EventUnifiedAppId = df.GetHeader(AppId)
		event.EventType = df.GetHeader(EventType)
		event.EventCorpId = df.GetHeader(CorpId)
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

func safeInvokeEventHandler(event *GenericOpenDingTalkEvent, handler GenericEventHandler) (status EventStatus) {
	defer func() {
		if e := recover(); e != nil {
			logger.GetLogger().Errorf("failed to invoke event handler, error=[%s]", e)
			status = EventStatusLater
		}
	}()
	return handler(event)
}
