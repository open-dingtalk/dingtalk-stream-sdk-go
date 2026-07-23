package client

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
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
	assert.Equal(t, "dingtalk-sdk-go/v0.9.2", c.UserAgent.UserAgent)
}

func TestWithKeepAlive(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		c := NewStreamClient()
		assert.Equal(t, 120*time.Second, c.keepAliveIdle)
	})

	t.Run("disable", func(t *testing.T) {
		c := NewStreamClient(WithKeepAlive(0))
		assert.Equal(t, time.Duration(0), c.keepAliveIdle)

		c = NewStreamClient(WithKeepAlive(-time.Second))
		assert.Equal(t, time.Duration(0), c.keepAliveIdle)
	})

	t.Run("raise short interval", func(t *testing.T) {
		c := NewStreamClient(WithKeepAlive(time.Second))
		assert.Equal(t, 3*time.Second, c.keepAliveIdle)
	})

	t.Run("custom", func(t *testing.T) {
		c := NewStreamClient(WithKeepAlive(30 * time.Second))
		assert.Equal(t, 30*time.Second, c.keepAliveIdle)
	})
}
