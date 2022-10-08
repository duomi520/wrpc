package wrpc

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

func (hi *hi) Order(ctx context.Context, s *Stream) error {
	log.Println(ctx.Value("charset"), ctx.Value("content"), ctx.Value("http-equiv"))
	i := 9
	for i < 25 {
		req, err := s.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := s.Send(i + 1); err != nil {
			return err
		}
		log.Println("s", i+1, req)
		i++
	}
	return nil
}

func TestClientStream(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.ErrorLevel, "")
	defer logger.Close()
	o := NewOptions(WithLogger(logger))
	s := NewService(o)
	s.TCPServer(context.TODO(), ":4567")
	hi := new(hi)
	err := s.RegisterRPC("hi", hi)
	if err != nil {
		t.Fatal(err.Error())
	}
	client, err := NewTCPClient(context.TODO(), "127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	meta := make(map[any]any)
	meta["charset"] = "utf-8"
	meta["content"] = "webkit"
	meta["http-equiv"] = "X-UA-Compatible"
	ctx := context.WithValue(context.Background(), ContextKey, meta)
	rspStream, err := client.NewStream(ctx, "hi.Order")
	if err != nil {
		t.Fatal(err.Error())
	}
	//TODO 发送顺序的bug
	time.Sleep(time.Second)
	n := 6
	// send
	for i := 0; i < n; i++ {
		if err := rspStream.Send(i); err != nil {
			t.Fatal(err.Error())
		}
	}
	// recv
	for i := 0; i < n; i++ {
		rsp, err := rspStream.Recv()
		if err != nil {
			t.Fatal(err.Error())
		}
		log.Println("c", i, rsp)
	}
	client.CloseStream(rspStream)
}

/*
2022/10/07 22:33:46 utf-8 webkit X-UA-Compatible
2022/10/07 22:33:47 s 10 0
2022/10/07 22:33:47 c 0 10
2022/10/07 22:33:47 s 11 2
2022/10/07 22:33:47 c 1 11
2022/10/07 22:33:47 s 12 1
2022/10/07 22:33:47 c 2 12
2022/10/07 22:33:47 c 3 13
2022/10/07 22:33:47 s 13 4
2022/10/07 22:33:47 s 14 5
2022/10/07 22:33:47 c 4 14
2022/10/07 22:33:47 s 15 3
2022/10/07 22:33:47 c 5 15
*/
