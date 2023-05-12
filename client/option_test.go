package client

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"testing"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:48
 */

func TestWithAppCredential(t *testing.T) {
	op := WithAppCredential(NewAppCredentialConfig("clientId", "clientSecret"))

	c := NewStreamClient(op)
	assert.Equal(t, "clientId", c.AppCredential.ClientId)
	assert.Equal(t, "clientSecret", c.AppCredential.ClientSecret)
}

func TestWithSubscription(t *testing.T) {
	op := WithSubscription("stype", "stopic", func(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
		return nil, nil
	})

	c := NewStreamClient(op)
	h, err := c.GetHandler("stype", "stopic")
	assert.Nil(t, err)
	assert.NotNil(t, h)
}

func TestWithUserAgent(t *testing.T) {
	op := WithUserAgent(NewDingtalkGoSDKUserAgent())
	c := NewStreamClient(op)
	assert.NotNil(t, c.UserAgent)
}
