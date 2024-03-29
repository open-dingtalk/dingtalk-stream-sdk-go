package main

// 插件例子
type EchoRequest struct {
	Message string
}
type EchoResponse struct {
	EchoMessage string
}

func Echo(echoRequest *EchoRequest) *EchoResponse {
	return &EchoResponse{
		EchoMessage: "You said: " + echoRequest.Message,
	}
}
