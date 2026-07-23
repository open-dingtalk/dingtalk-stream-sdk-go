package client

import (
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"
	"time"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:48
 */

type ClientOption func(*StreamClient)

func WithAutoReconnect(autoReconnect bool) ClientOption {
	return func(c *StreamClient) {
		c.AutoReconnect = autoReconnect
	}
}

func WithAppCredential(cred *AppCredentialConfig) ClientOption {
	return func(c *StreamClient) {
		c.AppCredential = cred
	}
}

func WithSubscription(stype, stopic string, frameHandler handler.IFrameHandler) ClientOption {
	return func(c *StreamClient) {
		c.RegisterRouter(stype, stopic, frameHandler)
	}
}

// WithKeepAlive sets idle duration before the client sends a websocket ping.
// Pass 0 (or negative) to disable client-side websocket ping and rely on
// application-level system/ping only. Values in (0, 3s) are raised to 3s.
func WithKeepAlive(keepAliveIdle time.Duration) ClientOption {
	return func(client *StreamClient) {
		if keepAliveIdle <= 0 {
			client.keepAliveIdle = 0
			return
		}
		if keepAliveIdle < 3*time.Second {
			keepAliveIdle = 3 * time.Second
		}
		client.keepAliveIdle = keepAliveIdle
	}
}

func WithUserAgent(ua *UserAgentConfig) ClientOption {
	return func(c *StreamClient) {
		if ua.Valid() != nil {
			ua = NewDingtalkGoSDKUserAgent()
		}

		c.UserAgent = ua
	}
}

func WithExtras(extras map[string]string) ClientOption {
	return func(c *StreamClient) {
		c.extras = extras
	}
}

func WithOpenApiHost(host string) ClientOption {
	return func(c *StreamClient) {
		c.openApiHost = host
	}
}

func WithProxy(proxy string) ClientOption {
	return func(c *StreamClient) {
		c.proxy = proxy
	}
}
