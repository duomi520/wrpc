package wrpc

import (
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

type hi struct {
}

func (hi *hi) SayHello(_ context.Context, args string) (string, error) {
	reply := "Hello " + args
	return reply, nil
}
func (hi *hi) Error(_ context.Context, args string) (string, error) {
	return "", errors.New("This a error.")
}
func (hi *hi) Meta(ctx context.Context, args string) (string, error) {
	log.Println(ctx.Value("charset"), ctx.Value("content"), ctx.Value("http-equiv"))
	return "", nil
}

func TestClientCall(t *testing.T) {
	o := NewOptions()
	s := NewService(o)
	s.TCPServer(context.TODO(), ":4567")
	hi := new(hi)
	s.RegisterRPC("hi", hi)
	client, err := NewTCPClient(context.TODO(), "127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	var reply string
	if err := client.Call(context.TODO(), "hi.SayHello", "linda", &reply); err != nil {
		t.Fatal(err.Error())
	}
	if !strings.EqualFold(reply, "Hello linda") {
		t.Fatal(reply)
	}
	if err := client.Call(context.TODO(), "hi.Error", "linda", &reply); err != nil {
		if !strings.EqualFold(err.Error(), "This a error.") {
			t.Fatal(reply)
		}
	}
	meta := make(map[any]any)
	meta["charset"] = "utf-8"
	meta["content"] = "webkit"
	meta["http-equiv"] = "X-UA-Compatible"
	ctx := context.WithValue(context.Background(), ContextKey, meta)
	if err := client.Call(ctx, "hi.Meta", "linda", &reply); err != nil {
		t.Fatal(err.Error())
	}
}
func TestClientGo(t *testing.T) {
	o := NewOptions()
	s := NewService(o)
	s.TCPServer(context.TODO(), ":4567")
	hi := new(hi)
	s.RegisterRPC("hi", hi)
	client, err := NewTCPClient(context.TODO(), "127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	var reply string
	done := make(chan struct{})
	err = client.Go(context.TODO(), "hi.SayHello", "linda", &reply, done)
	if err != nil {
		t.Fatal(err.Error())
	}
	<-done
	if !strings.EqualFold(reply, "Hello linda") {
		t.Fatal(reply)
	}
}
func TestClientClose(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	o := NewOptions(WithLogger(logger))
	ctx, ctxExitFunc := context.WithCancel(context.Background())
	s := NewService(o)
	s.TCPServer(ctx, ":4567")
	hi := new(hi)
	s.RegisterRPC("hi", hi)
	client, err := NewTCPClient(context.TODO(), "127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	client.Close()
	time.Sleep(250 * time.Millisecond)
	ctxExitFunc()
	time.Sleep(time.Second)
}

/*
[Debug] 2022-10-07 17:37:34 TCP监听端口:4567
[Debug] 2022-10-07 17:37:34 TCP已初始化连接，等待客户端连接……
[Debug] 2022-10-07 17:37:34 127.0.0.1:4567 tcpClientReceive stop
[Debug] 2022-10-07 17:37:34 127.0.0.1:4567 tcpSend stop
[Debug] 2022-10-07 17:37:34 127.0.0.1:61158 tcpServerReceive stop
[Debug] 2022-10-07 17:37:34 TCP等待子协程关闭……
[Debug] 2022-10-07 17:37:34 TCPServer关闭。
[Debug] 2022-10-07 17:37:34 TCP监听端口关闭。
*/
