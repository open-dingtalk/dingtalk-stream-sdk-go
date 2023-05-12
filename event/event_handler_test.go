package event

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

/**
 * @Author linya.jj
 * @Date 2023/4/27 09:25
 */

func TestEventHandlerDoNothing(t *testing.T) {
	status, err := EventHandlerDoNothing(context.Background(), nil, []byte(""))
	assert.Nil(t, err)
	assert.Equal(t, EventProcessStatusKSuccess, status)
}

func TestEventHandlerSaveToRDS(t *testing.T) {
	status, err := EventHandlerSaveToRDS(context.Background(), nil, []byte(""))
	assert.Nil(t, err)
	assert.Equal(t, EventProcessStatusKSuccess, status)
}
