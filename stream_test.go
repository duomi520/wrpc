package wrpc

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

func (hi *hi) Order(ctx context.Context, s *Stream) error {
	log.Println(GetMetadata(ctx))
	i := 9
	for i < 25 {
		req, err := s.Recv()
		if err != nil {
			log.Println(err.Error())
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
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.ErrorLevel, "")
	defer logger.Close()
	o := NewOptions(WithLogger(logger))
	s := NewService(o)
	s.TCPServer(":4567")
	defer s.Stop()
	hi := new(hi)
	err := s.RegisterRPC("hi", hi)
	if err != nil {
		t.Fatal(err.Error())
	}
	client, err := NewTCPClient("127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer client.Close()
	var meta utils.MetaDict
	meta.Set("charset", "utf-8")
	meta.Set("content", "webkit")
	meta.Set("http-equiv", "X-UA-Compatible")
	ctx := MetadataContext(context.Background(), &meta)
	rspStream, err := client.NewStream(ctx, "hi.Order")
	if err != nil {
		t.Fatal(err.Error())
	}
	time.Sleep(250 * time.Millisecond)
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
2022/12/03 22:01:46 &{3 [charset content http-equiv] [utf-8 webkit X-UA-Compatible]}
2022/12/03 22:01:46 c 0 10
2022/12/03 22:01:46 s 10 0
2022/12/03 22:01:46 s 11 1
2022/12/03 22:01:46 s 12 2
2022/12/03 22:01:46 c 1 11
2022/12/03 22:01:46 c 2 12
2022/12/03 22:01:46 s 13 3
2022/12/03 22:01:46 c 3 13
2022/12/03 22:01:46 s 14 4
2022/12/03 22:01:46 s 15 5
2022/12/03 22:01:46 c 4 14
2022/12/03 22:01:46 c 5 15
2022/12/03 22:01:46 context canceled
*/

func TestStreamCtxCancelFunc(t *testing.T) {
	Default()
	defer Stop()
	o := NewOptions()
	s := NewService(o)
	s.TCPServer(":4567")
	defer s.Stop()
	hi := new(hi)
	err := s.RegisterRPC("hi", hi)
	if err != nil {
		t.Fatal(err.Error())
	}
	client, err := NewTCPClient("127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	rspStream, err := client.NewStream(ctx, "hi.Order")
	if err != nil {
		t.Fatal(err.Error())
	}
	time.Sleep(250 * time.Millisecond)
	cancel()
	client.CloseStream(rspStream)
}

/*
2022/12/03 22:02:12 <nil>
2022/12/03 22:02:12 context canceled
*/
