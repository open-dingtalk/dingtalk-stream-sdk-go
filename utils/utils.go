package utils

/**
 * @Author linya.jj
 * @Date 2023/3/22 15:23
 */

const (
	OpenDingTalkEndpoint = "https://api.dingtalk.com"
	OpenConnectionUri    = "/v1.0/gateway/connections/open"
)

const (
	SubscriptionTypeKSystem   = "SYSTEM"   //系统请求
	SubscriptionTypeKEvent    = "EVENT"    //事件
	SubscriptionTypeKCallback = "CALLBACK" //回调
)

var (
	SubscriptionTypeSet = map[string]bool{
		SubscriptionTypeKSystem:   true,
		SubscriptionTypeKEvent:    true,
		SubscriptionTypeKCallback: true,
	}
)
