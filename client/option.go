package client

import "github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"

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
		if c.subscriptions == nil {
			c.subscriptions = make(map[string]map[string]handler.IFrameHandler)
		}

		if _, ok := c.subscriptions[stype]; !ok {
			c.subscriptions[stype] = make(map[string]handler.IFrameHandler)
		}

		c.subscriptions[stype][stopic] = frameHandler
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
