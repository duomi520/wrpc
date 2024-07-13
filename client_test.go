package wrpc

import (
	"context"
	"errors"

	//"fmt"
	"log"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

type hi struct {
}

func (hi *hi) SayHello(_ *RPCContext, args string) (string, error) {
	reply := "Hello " + args
	return reply, nil
}
func (hi *hi) Error(_ *RPCContext, args string) (string, error) {
	return "", errors.New("This a error.")
}
func (hi *hi) Meta(ctx *RPCContext, args string) (string, error) {
	m := ctx.Metadata
	log.Println(m.GetAll())
	return "", nil
}

func TestClientCall(t *testing.T) {
	o := NewOptions()
	s := NewService(o)
	s.TCPServer(":4567")
	defer s.Stop()
	hi := new(hi)
	s.RegisterRPC("hi", hi)
	client, err := NewTCPClient("127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer client.Close()
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
	var meta utils.MetaDict[string]
	meta = meta.Set("charset", "utf-8")
	meta = meta.Set("content", "webkit")
	meta = meta.Set("http-equiv", "X-UA-Compatible")
	ctx := SetMeta(context.TODO(), meta)
	if err := client.Call(ctx, "hi.Meta", "linda", &reply); err != nil {
		t.Fatal(err.Error())
	}
}

/*
2024/07/13 20:29:13 [utf-8 webkit X-UA-Compatible]
*/
/*
	func TestClientGo(t *testing.T) {
		StartGuardian()
		defer StopGuardian()
		o := NewOptions()
		s := NewService(o)
		s.TCPServer(":4567")
		defer s.Stop()
		hi := new(hi)
		s.RegisterRPC("hi", hi)
		client, err := NewTCPClient("127.0.0.1:4567", o)
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
*/
func TestClientClose(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	o := NewOptions(WithLogger(slog.Default()))
	s := NewService(o)
	s.TCPServer(":4567")
	s.RegisterRPC("hi", new(hi))
	client, err := NewTCPClient("127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	client.Close()
	time.Sleep(250 * time.Millisecond)
	s.Stop()
	time.Sleep(time.Second)
}

/*
2024/07/13 20:29:26 DEBUG TCPServer.Run:TCP监听端口 :4567
2024/07/13 20:29:26 DEBUG TCPServer.Run:TCP已初始化连接,等待客户端连接……
2024/07/13 20:29:26 DEBUG TCPSession.tcpClientSend:127.0.0.1:4567 开启
2024/07/13 20:29:26 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4567 开启
2024/07/13 20:29:26 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4567 关闭
2024/07/13 20:29:26 DEBUG TCPSession.tcpClientSend:127.0.0.1:4567 关闭
2024/07/13 20:29:26 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51499 开启
2024/07/13 20:29:26 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51499 关闭
2024/07/13 20:29:26 DEBUG TCPServer.Run:TCPServer停止接受新连接,等待子协程关闭……
2024/07/13 20:29:26 DEBUG TCPServer.Run:TCPServer关闭
2024/07/13 20:29:26 DEBUG TCPServer.Stop:TCP监听端口关闭
*/
