package event

import (
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

/**
 * @Author linya.jj
 * @Date 2023/4/26 17:15
 */

func TestNewEventHeaderFromDataFrame(t *testing.T) {
	assert.NotNil(t, NewEventHeaderFromDataFrame(nil))

	df := &payload.DataFrame{
		SpecVersion: "version",
		Type:        utils.SubscriptionTypeKEvent,
		Time:        12345678,
		Headers: payload.DataFrameHeader{
			DataFrameHeaderKEventId:           "eventId",
			DataFrameHeaderKEventBornTime:     "1234567890",
			DataFrameHeaderKEventCorpId:       "eventCorpId",
			DataFrameHeaderKEventType:         "eventType",
			DataFrameHeaderKEventUnifiedAppId: "eventUnifiedAppId",
		},
		Data: "",
	}

	eh := NewEventHeaderFromDataFrame(df)
	assert.NotNil(t, eh)
	assert.Equal(t, "eventId", eh.EventId)
	assert.Equal(t, int64(1234567890), eh.EventBornTime)
	assert.Equal(t, "eventCorpId", eh.EventCorpId)
	assert.Equal(t, "eventType", eh.EventType)
	assert.Equal(t, "eventUnifiedAppId", eh.EventUnifiedAppId)
}

func TestNewEventProcessResultSuccess(t *testing.T) {
	assert.NotNil(t, NewEventProcessResultSuccess())
}

func TestNewEventProcessResultLater(t *testing.T) {
	assert.NotNil(t, NewEventProcessResultLater())
}
