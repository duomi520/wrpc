package wrpc

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

func TestTCPDial(t *testing.T) {
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	h := func([]byte, WriterFunc, func()) error { return nil }
	s := NewTCPServer(":4567", h, logger)
	defer s.Stop()
	go s.Run()
	c, err := TCPDial("127.0.0.1:4567", defaultProtocolMagicNumber, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	atomic.StoreInt32(c.stopSign, 1)
	c.Send(frameGoaway)
	time.Sleep(1 * time.Second)
}

/*
[Debug] 2022-12-03 22:02:27 TCPServer.Run：TCP监听端口:4567
[Debug] 2022-12-03 22:02:27 TCPServer.Run：TCP已初始化连接，等待客户端连接……
[Debug] 2022-12-03 22:02:27 TCPSession.tcpClientReceive: 127.0.0.1:4567 stop
[Debug] 2022-12-03 22:02:27 TCPSession.tcpServerReceive: 127.0.0.1:61018 stop
[Debug] 2022-12-03 22:02:28 TCPServer.Run: TCP等待子协程关闭……
[Debug] 2022-12-03 22:02:28 TCPServer.Run: TCPServer关闭。
[Debug] 2022-12-03 22:02:28 TCPServer.Stop：TCP监听端口关闭。
*/

func TestTCPGracefulStop(t *testing.T) {
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	h := func([]byte, WriterFunc, func()) error { return nil }
	s := NewTCPServer(":4568", h, logger)
	go s.Run()
	_, err := TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	_, err = TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	s.Stop()
	time.Sleep(5 * time.Second)
}

/*
[Debug] 2022-12-03 22:02:47 TCPServer.Run：TCP监听端口:4568
[Debug] 2022-12-03 22:02:47 TCPServer.Run：TCP已初始化连接，等待客户端连接……
[Debug] 2022-12-03 22:02:48 TCPServer.Stop：TCP监听端口关闭。
[Debug] 2022-12-03 22:02:48 TCPServer.Run: TCP等待子协程关闭……
[Debug] 2022-12-03 22:02:52 TCPSession.tcpClientReceive: 127.0.0.1:4568 stop
[Debug] 2022-12-03 22:02:52 TCPSession.tcpServerReceive: 127.0.0.1:61024 stop
[Debug] 2022-12-03 22:02:53 TCPSession.tcpServerReceive: 127.0.0.1:61025 stop
[Debug] 2022-12-03 22:02:53 TCPSession.tcpClientReceive: 127.0.0.1:4568 stop
[Debug] 2022-12-03 22:02:53 TCPServer.Run: TCPServer关闭。
*/
func TestTCPEcho(t *testing.T) {
	Default()
	defer Stop()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	var num, count int
	//50000
	loop := 50000
	stop := make(chan struct{})
	hs := func(msg []byte, send WriterFunc, f func()) error {
		data := make([]byte, len(msg))
		copy(data, msg)
		send(data)
		return nil
	}
	s := NewTCPServer(":4568", hs, logger)
	go s.Run()
	hc := func(msg []byte, send WriterFunc, f func()) error {
		n := int(utils.BytesToInteger32[uint32](msg[6:10]))
		num++
		count += n
		if num == loop {
			time.Sleep(250 * time.Millisecond)
			close(stop)
		}
		return nil
	}
	c, err := TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, hc, logger)
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
	s.Stop()
	atomic.StoreInt32(c.stopSign, 1)
	time.Sleep(5 * time.Second)
	if count != (loop-1)*loop/2 {
		logger.Debug(strconv.Itoa(count), " <> ", strconv.Itoa((loop-1)*loop/2))
		t.Fatal("count err")
	}
	logger.Debug(strconv.Itoa(count), " = ", strconv.Itoa((loop-1)*loop/2))
}

/*
[Debug] 2022-12-03 22:03:03 TCPServer.Run：TCP监听端口:4568
[Debug] 2022-12-03 22:03:03 TCPServer.Run：TCP已初始化连接，等待客户端连接……
[Debug] 2022-12-03 22:03:04 TCPServer.Run: TCP等待子协程关闭……
[Debug] 2022-12-03 22:03:04 TCPServer.Stop：TCP监听端口关闭。
[Debug] 2022-12-03 22:03:08 TCPSession.tcpPing: 127.0.0.1:4568 stop
[Debug] 2022-12-03 22:03:08 TCPSession.tcpClientReceive: 127.0.0.1:4568 stop
[Debug] 2022-12-03 22:03:08 TCPSession.tcpServerReceive: 127.0.0.1:61029 stop
[Debug] 2022-12-03 22:03:08 TCPServer.Run: TCPServer关闭。
[Debug] 2022-12-03 22:03:09 1249975000 = 1249975000
*/
