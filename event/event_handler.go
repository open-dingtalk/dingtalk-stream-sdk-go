package event

import (
	"context"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
)

/**
 * @Author linya.jj
 * @Date 2023/4/27 09:25
 */

type IEventHandler func(c context.Context, header *EventHeader, rawData []byte) (EventProcessStatusType, error)

func EventHandlerDoNothing(c context.Context, header *EventHeader, rawData []byte) (EventProcessStatusType, error) {
	logger.GetLogger().Debugf("EventHandlerDoNothing header=[%s], rawData=[%s]",
		header, rawData)

	return EventProcessStatusKSuccess, nil
}

func EventHandlerSaveToRDS(c context.Context, header *EventHeader, rawData []byte) (EventProcessStatusType, error) {
	// TODO save data to rds here

	return EventProcessStatusKSuccess, nil
}
