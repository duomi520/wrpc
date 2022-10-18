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
	m := GetMetadata(ctx)
	log.Println(m.GetAll())
	return "", nil
}

func TestClientCall(t *testing.T) {
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
	var meta utils.MetaDict
	meta.Set("charset", "utf-8")
	meta.Set("content", "webkit")
	meta.Set("http-equiv", "X-UA-Compatible")
	ctx := MetadataContext(context.Background(), &meta)
	if err := client.Call(ctx, "hi.Meta", "linda", &reply); err != nil {
		t.Fatal(err.Error())
	}
}

/*
2022/10/18 20:41:30 [charset content http-equiv] [utf-8 webkit X-UA-Compatible]
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
	StartGuardian()
	defer StopGuardian()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	o := NewOptions(WithLogger(logger))
	s := NewService(o)
	s.TCPServer(":4567")
	hi := new(hi)
	s.RegisterRPC("hi", hi)
	client, err := NewTCPClient("127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	client.Close()
	time.Sleep(250 * time.Millisecond)
	s.Stop()
	time.Sleep(6 * time.Second)
}

/*
[Info ] 2022-10-17 20:58:46 Pid: 90012
[Debug] 2022-10-17 20:58:46 TCP监听端口:4567
[Debug] 2022-10-17 20:58:46 TCP已初始化连接，等待客户端连接……
[Debug] 2022-10-17 20:58:46 127.0.0.1:4567 tcpClientReceive stop
[Debug] 2022-10-17 20:58:46 127.0.0.1:61949 tcpServerReceive stop
[Debug] 2022-10-17 20:58:47 TCP等待子协程关闭……
[Debug] 2022-10-17 20:58:47 TCPServer关闭。
[Debug] 2022-10-17 20:58:47 TCP监听端口关闭。
[Debug] 2022-10-17 20:58:51 127.0.0.1:4567 tcpPing stop
*/
