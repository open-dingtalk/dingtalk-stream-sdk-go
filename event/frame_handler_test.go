package event

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"testing"
)

/**
 * @Author linya.jj
 * @Date 2023/4/26 17:15
 */

func EventHandlerSuccess(c context.Context, header *EventHeader, rawData []byte) (EventProcessStatusType, error) {
	return EventProcessStatusKSuccess, nil
}

func EventHandlerLater(c context.Context, header *EventHeader, rawData []byte) (EventProcessStatusType, error) {
	return EventProcessStatusKLater, nil
}

func EventHandlerLaterError(c context.Context, header *EventHeader, rawData []byte) (EventProcessStatusType, error) {
	return EventProcessStatusKLater, errors.New("error")
}

func TestDefaultEventFrameHandler_OnEventReceived(t *testing.T) {
	defh := NewDefaultEventFrameHandler(nil)
	ret, err := defh.OnEventReceived(context.Background(), nil)
	assert.Nil(t, ret)
	assert.Nil(t, err)

	df := &payload.DataFrame{}

	defh = NewDefaultEventFrameHandler(EventHandlerSuccess)
	ret, err = defh.OnEventReceived(context.Background(), df)
	assert.Equal(t, payload.DataFrameResponseStatusCodeKOK, ret.Code)
	assert.Nil(t, err)

	defh = NewDefaultEventFrameHandler(EventHandlerLater)
	ret, err = defh.OnEventReceived(context.Background(), df)
	assert.Equal(t, payload.DataFrameResponseStatusCodeKInternalError, ret.Code)
	assert.Nil(t, err)

	defh = NewDefaultEventFrameHandler(EventHandlerLaterError)
	ret, err = defh.OnEventReceived(context.Background(), df)
	assert.Equal(t, payload.DataFrameResponseStatusCodeKInternalError, ret.Code)
	assert.Nil(t, err)
}
