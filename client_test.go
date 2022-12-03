package wrpc

import (
	"context"
	"errors"
	"fmt"
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
func (hi *hi) CancelFunc(ctx context.Context, args string) (string, error) {
	log.Println("wait ctx cancel")
	<-ctx.Done()
	log.Println("ctx done")
	return "", nil
}

func TestClientCall(t *testing.T) {
	Default()
	defer Stop()
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
func TestCtxCancelFunc(t *testing.T) {
	Default()
	defer Stop()
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
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Second)
		cancel()
	}()
	if err := client.Call(ctx, "hi.CancelFunc", "hi", &reply); err != nil {
		if !strings.EqualFold(err.Error(), "context canceled") {
			t.Fatal(err.Error())
		}
	}
	time.Sleep(250 * time.Millisecond)
}

/*
2022/10/22 23:00:39 wait ctx cancel
2022/10/22 23:00:40 ctx done
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
	Default()
	defer Stop()
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
[Debug] 2022-12-03 22:00:08 TCPServer.Run：TCP监听端口:4567
[Debug] 2022-12-03 22:00:08 TCPServer.Run：TCP已初始化连接，等待客户端连接……
[Debug] 2022-12-03 22:00:08 TCPSession.tcpClientReceive: 127.0.0.1:4567 stop
[Debug] 2022-12-03 22:00:08 TCPSession.tcpServerReceive: 127.0.0.1:60960 stop
[Debug] 2022-12-03 22:00:08 TCPServer.Stop：TCP监听端口关闭。
[Debug] 2022-12-03 22:00:08 TCPServer.Run: TCP等待子协程关闭……
[Debug] 2022-12-03 22:00:08 TCPServer.Run: TCPServer关闭。
[Debug] 2022-12-03 22:00:13 TCPSession.tcpPing: 127.0.0.1:4567 stop
*/
func TestHijacker(t *testing.T) {
	Default()
	defer Stop()
	oc := NewOptions(WithHijacker(func(data []byte, send WriterFunc) error {
		log.Print("Client: ", string(data))
		return nil
	}))
	os := NewOptions(WithHijacker(func(data []byte, send WriterFunc) error {
		log.Print("Server: ", string(data))
		HijackerSend([]byte("123456收到"), send)
		return nil
	}))
	s := NewService(os)
	s.TCPServer(":4567")
	c, err := NewTCPClient("127.0.0.1:4567", oc)
	if err != nil {
		t.Fatal(err.Error())
	}
	HijackerSend([]byte("123456hi"), c.Send)
	HijackerSend([]byte("123456jackey"), c.Send)
	time.Sleep(250 * time.Millisecond)
	c.Close()
	s.Stop()
	time.Sleep(1 * time.Second)
}

/*
2022/12/03 22:00:37 Server: hi
2022/12/03 22:00:37 Server: jackey
2022/12/03 22:00:37 Client: 收到
2022/12/03 22:00:37 Client: 收到
*/
func TestClientHook(t *testing.T) {
	Default()
	defer Stop()
	s := NewService(NewOptions())
	s.TCPServer(":4567")
	defer s.Stop()
	hi := new(hi)
	s.RegisterRPC("hi", hi)
	o := NewOptions()
	intelA := func(b []byte, w WriterFunc) ([]byte, error) {
		fmt.Println("A", b)
		return b, nil
	}
	intelB := func(b []byte, w WriterFunc) ([]byte, error) {
		fmt.Println("B", b)
		return b, nil
	}
	outlet := func(b []byte, w WriterFunc) ([]byte, error) {
		fmt.Println("O", b)
		return b, nil
	}
	o.IntletHook = append(o.IntletHook, intelA, intelB)
	o.OutletHook = append(o.OutletHook, outlet)
	client, err := NewTCPClient("127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	var reply string
	if err := client.Call(context.TODO(), "hi.SayHello", "linda", &reply); err != nil {
		t.Fatal(err.Error())
	}
}

/*
O [37 0 0 0 16 0 0 0 128 194 89 106 219 255 29 0 29 0 104 105 46 83 97 121 72 101 108 108 111 34 108 105 110 100 97 34 10]
A [43 0 0 0 17 0 0 0 128 194 89 106 219 255 29 0 29 0 104 105 46 83 97 121 72 101 108 108 111 34 72 101 108 108 111 32 108 105 110 100 97 34 10]
B [43 0 0 0 17 0 0 0 128 194 89 106 219 255 29 0 29 0 104 105 46 83 97 121 72 101 108 108 111 34 72 101 108 108 111 32 108 105 110 100 97 34 10]
*/
