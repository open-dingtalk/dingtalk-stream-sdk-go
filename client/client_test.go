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

func newTestWebsocketConn(t *testing.T, serverHandler func(*websocket.Conn)) *websocket.Conn {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		go serverHandler(conn)
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		server.Close()
	})

	return conn
}

func runProcessLoop(t *testing.T, conn *websocket.Conn, keepAliveIdle time.Duration) <-chan struct{} {
	t.Helper()

	cli := &StreamClient{
		conn:          conn,
		AutoReconnect: false,
		keepAliveIdle: keepAliveIdle,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.processLoop()
	}()
	return done
}

func waitProcessLoopExit(t *testing.T, done <-chan struct{}, timeout time.Duration) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("processLoop did not exit")
	}
}

func TestDingtalkOpenStreamClient_processLoop_ReadErrorCancelsLoop(t *testing.T) {
	conn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		_ = conn.Close()
	})

	done := runProcessLoop(t, conn, time.Hour)

	waitProcessLoopExit(t, done, time.Second)
}

func TestDingtalkOpenStreamClient_processLoop_PingTimeoutCancelsLoop(t *testing.T) {
	conn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		conn.SetPingHandler(func(string) error {
			return nil
		})
		for {
			if _, _, err := conn.NextReader(); err != nil {
				_ = conn.Close()
				return
			}
		}
	})

	done := runProcessLoop(t, conn, 10*time.Millisecond)

	waitProcessLoopExit(t, done, 6*time.Second)
}

func TestDingtalkOpenStreamClient_processLoop_PongHandlerAfterExitDoesNotPanic(t *testing.T) {
	conn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		_ = conn.Close()
	})

	done := runProcessLoop(t, conn, time.Hour)

	waitProcessLoopExit(t, done, time.Second)

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
