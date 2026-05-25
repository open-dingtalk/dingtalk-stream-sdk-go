package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:23
 */

func TestNewDingtalkOpenStreamClient(t *testing.T) {
}

func TestDingtalkOpenStreamClient_Start(t *testing.T) {
}

func TestDingtalkOpenStreamClient_processDataFrame(t *testing.T) {
}

func TestDingtalkOpenStreamClient_processLoop_PongHandlerAfterExitDoesNotPanic(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	cli := &StreamClient{
		conn:          conn,
		AutoReconnect: false,
		keepAliveIdle: time.Hour,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.processLoop()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processLoop did not exit")
	}

	pongHandler := conn.PongHandler()
	for i := 0; i < 100; i++ {
		func() {
			defer func() {
				if err := recover(); err != nil {
					t.Fatalf("pong handler panicked after processLoop exit: %v", err)
				}
			}()
			if err := pongHandler(""); err != nil {
				t.Fatalf("pong handler returned error: %v", err)
			}
		}()
	}
}

func TestDingtalkOpenStreamClient_Close(t *testing.T) {

}

func TestDingtalkOpenStreamClient_reconnect(t *testing.T) {
}

func TestDingtalkOpenStreamClient_GetHandler(t *testing.T) {
}

func TestDingtalkOpenStreamClient_CheckConfigValid(t *testing.T) {
}

func TestDingtalkOpenStreamClient_GetConnectionEndpoint(t *testing.T) {

}

func TestDingtalkOpenStreamClient_OnDisconnect(t *testing.T) {

}

func TestDingtalkOpenStreamClient_OnPing(t *testing.T) {
}

func TestDingtalkOpenStreamClient_SendDataFrameResponse(t *testing.T) {
}

func TestDingtalkOpenStreamClient_SendErrorResponse(t *testing.T) {
}
