package wrpc

import (
	"context"
	"log"
	"log/slog"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

func (hi *hi) Order(ctx context.Context, s *Stream) error {
	log.Println(GetMeta(ctx))
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
	slog.SetLogLoggerLevel(slog.LevelError)
	o := NewOptions(WithLogger(slog.Default()))
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
	var meta utils.MetaDict[string]
	meta = meta.Set("charset", "utf-8")
	meta = meta.Set("content", "webkit")
	meta = meta.Set("http-equiv", "X-UA-Compatible")
	ctx := SetMeta(context.Background(), meta)
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
2024/07/13 20:31:23 {[9961587957729687531 4759308327122588290 10397283276275993222] [utf-8 webkit X-UA-Compatible]}
2024/07/13 20:31:24 s 10 0
2024/07/13 20:31:24 s 11 1
2024/07/13 20:31:24 c 0 10
2024/07/13 20:31:24 c 1 11
2024/07/13 20:31:24 s 12 2
2024/07/13 20:31:24 s 13 3
2024/07/13 20:31:24 s 14 4
2024/07/13 20:31:24 c 2 12
2024/07/13 20:31:24 c 3 13
2024/07/13 20:31:24 c 4 14
2024/07/13 20:31:24 s 15 5
2024/07/13 20:31:24 c 5 15
2024/07/13 20:31:24 context canceled
*/

func TestStreamCtxCancelFunc(t *testing.T) {
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
2024/07/13 20:31:51 {[] []}
2024/07/13 20:31:51 context canceled
*/
