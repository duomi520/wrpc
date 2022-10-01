package wrpc

import (
	"context"
	"github.com/duomi520/utils"
	"strconv"
	"testing"
	"time"
)

func TestTCPDial(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	h := func(send func([]byte) error, msg []byte) error { return nil }
	s := NewTCPServer(context.TODO(), ":4567", h, logger)
	go s.Run()
	ctx, ctxExitFunc := context.WithCancel(context.Background())
	_, err := TCPDial(ctx, "127.0.0.1:4567", defaultProtocolMagicNumber, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	ctxExitFunc()
	time.Sleep(100 * time.Millisecond)
}

/*
[Debug] 2022-01-09 22:24:42 TCP监听端口:4567
[Debug] 2022-01-09 22:24:42 TCP已初始化连接，等待客户端连接……
[Debug] 2022-01-09 22:24:43 127.0.0.1:4567 tcpSend stop
[Debug] 2022-01-09 22:24:43 127.0.0.1:54612 tcpReceive stop
[Debug] 2022-01-09 22:24:43 127.0.0.1:4567 tcpReceive stop
*/

func TestTCPGracefulStop(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	ctx, ctxExitFunc := context.WithCancel(context.Background())
	h := func(send func([]byte) error, msg []byte) error { return nil }
	s := NewTCPServer(ctx, ":4568", h, logger)
	go s.Run()
	_, err := TCPDial(context.TODO(), "127.0.0.1:4568", defaultProtocolMagicNumber, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	_, err = TCPDial(context.TODO(), "127.0.0.1:4568", defaultProtocolMagicNumber, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	ctxExitFunc()
	time.Sleep(5 * time.Second)
}

/*
[Debug] 2022-01-09 23:30:18 TCP监听端口:4568
[Debug] 2022-01-09 23:30:18 TCP已初始化连接，等待客户端连接……
[Debug] 2022-01-09 23:30:18 TCP等待子协程关闭……
[Debug] 2022-01-09 23:30:18 TCP监听端口关闭。
[Debug] 2022-01-09 23:30:22 127.0.0.1:51260 tcpReceive stop
[Debug] 2022-01-09 23:30:22 127.0.0.1:51259 tcpReceive stop
[Debug] 2022-01-09 23:30:22 127.0.0.1:4568 tcpSend stop
[Debug] 2022-01-09 23:30:22 127.0.0.1:4568 tcpReceive stop
[Debug] 2022-01-09 23:30:22 TCPServer关闭。
[Debug] 2022-01-09 23:30:22 127.0.0.1:4568 tcpReceive stop
[Debug] 2022-01-09 23:30:22 127.0.0.1:4568 tcpSend stop
*/
func TestTCPEcho(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	var num, count int
	//50000
	loop := 50000
	stop := make(chan struct{})
	ctx, ctxExitFunc := context.WithCancel(context.Background())
	hs := func(send func([]byte) error, msg []byte) error {
		data := make([]byte, len(msg))
		copy(data, msg)
		send(data)
		return nil
	}
	s := NewTCPServer(ctx, ":4568", hs, logger)
	go s.Run()
	hc := func(send func([]byte) error, msg []byte) error {
		n := int(utils.BytesToInteger32[uint32](msg[6:10]))
		num++
		count += n
		if num == loop {
			time.Sleep(250 * time.Millisecond)
			close(stop)
		}
		return nil
	}
	c, err := TCPDial(ctx, "127.0.0.1:4568", defaultProtocolMagicNumber, hc, logger)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 10)
	utils.CopyInteger32(data[0:4], uint32(10))
	for i := 0; i < loop; i++ {
		utils.CopyInteger32(data[6:10], uint32(i))
		c.Send(data)
	}
	<-stop
	time.Sleep(250 * time.Millisecond)
	ctxExitFunc()
	time.Sleep(1000 * time.Millisecond)
	if count != (loop-1)*loop/2 {
		logger.Debug(strconv.Itoa(count), " <> ", strconv.Itoa((loop-1)*loop/2))
		t.Fatal("count err")
	}
	logger.Debug(strconv.Itoa(count), " = ", strconv.Itoa((loop-1)*loop/2))
}

/*
[Debug] 2022-01-10 21:31:25 TCP监听端口:4568
[Debug] 2022-01-10 21:31:25 TCP已初始化连接，等待客户端连接……
[Debug] 2022-01-10 21:31:26 127.0.0.1:4568 tcpSend stop
[Debug] 2022-01-10 21:31:26 TCP监听端口关闭。
[Debug] 2022-01-10 21:31:26 127.0.0.1:4568 tcpReceive stop
[Debug] 2022-01-10 21:31:26 127.0.0.1:53167 tcpReceive stop
[Debug] 2022-01-10 21:31:26 TCP等待子协程关闭……
[Debug] 2022-01-10 21:31:26 TCPServer关闭。
[Debug] 2022-01-10 21:31:27 1249975000 = 1249975000
*/
