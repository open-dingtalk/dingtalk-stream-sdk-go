package event

import (
	"strconv"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

/**
 * @Author linya.jj
 * @Date 2023/4/26 17:15
 */

const (
	DataFrameHeaderKEventId           = "eventId"
	DataFrameHeaderKEventBornTime     = "eventBornTime"
	DataFrameHeaderKEventCorpId       = "eventCorpId"
	DataFrameHeaderKEventType         = "eventType"
	DataFrameHeaderKEventUnifiedAppId = "eventUnifiedAppId"
)

type EventHeader struct {
	EventId           string `json:"eventId"`
	EventBornTime     int64  `json:"eventBornTime"`
	EventCorpId       string `json:"eventCorpId"`
	EventType         string `json:"eventType"`
	EventUnifiedAppId string `json:"eventUnifiedAppId"`
}

func NewEventHeaderFromDataFrame(df *payload.DataFrame) *EventHeader {
	if df == nil {
		return &EventHeader{}
	}

	eventHeader := &EventHeader{
		EventId:           df.GetHeader(DataFrameHeaderKEventId),
		EventBornTime:     0,
		EventCorpId:       df.GetHeader(DataFrameHeaderKEventCorpId),
		EventType:         df.GetHeader(DataFrameHeaderKEventType),
		EventUnifiedAppId: df.GetHeader(DataFrameHeaderKEventUnifiedAppId),
	}

	if ts, err := strconv.ParseInt(df.GetHeader(DataFrameHeaderKEventBornTime), 10, 64); err == nil {
		eventHeader.EventBornTime = ts
	}

	return eventHeader
}

type EventProcessStatusType string

var (
	EventProcessStatusKSuccess EventProcessStatusType = "SUCCESS"
	EventProcessStatusKLater   EventProcessStatusType = "LATER"
)

type EventProcessResult struct {
	Status  EventProcessStatusType `json:"status"`
	Message string                 `json:"message"`
}

func NewEventProcessResultSuccess() *EventProcessResult {
	return &EventProcessResult{
		Status:  EventProcessStatusKSuccess,
		Message: "success",
	}
}

func NewEventProcessResultLater() *EventProcessResult {
	return &EventProcessResult{
		Status:  EventProcessStatusKLater,
		Message: "later",
	}
}
