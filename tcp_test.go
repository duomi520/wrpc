package wrpc

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

func TestTCPDial(t *testing.T) {
	StartGuardian()
	defer StopGuardian()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	h := func([]byte, func([]byte) error) error { return nil }
	s := NewTCPServer(":4567", nil, h, logger)
	defer s.Stop()
	go s.Run()
	c, err := TCPDial("127.0.0.1:4567", defaultProtocolMagicNumber, nil, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	atomic.StoreInt32(c.stopSign, 1)
	c.Send(frameGoaway)
	time.Sleep(1 * time.Second)
}

/*
[Debug] 2022-10-17 20:37:57 TCP监听端口:4567
[Debug] 2022-10-17 20:37:57 TCP已初始化连接，等待客户端连接……
[Debug] 2022-10-17 20:37:57 127.0.0.1:4567 tcpClientReceive stop
[Debug] 2022-10-17 20:37:57 127.0.0.1:56682 tcpServerReceive stop
[Debug] 2022-10-17 20:37:58 TCP等待子协程关闭……
[Debug] 2022-10-17 20:37:58 TCPServer关闭。
[Debug] 2022-10-17 20:37:58 TCP监听端口关闭。
*/

func TestTCPGracefulStop(t *testing.T) {
	StartGuardian()
	defer StopGuardian()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	h := func([]byte, func([]byte) error) error { return nil }
	s := NewTCPServer(":4568", nil, h, logger)
	go s.Run()
	_, err := TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, nil, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	_, err = TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, nil, h, logger)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	s.Stop()
	time.Sleep(5 * time.Second)
}

/*
[Debug] 2022-10-17 20:40:10 TCP监听端口:4568
[Debug] 2022-10-17 20:40:10 TCP已初始化连接，等待客户端连接……
[Debug] 2022-10-17 20:40:10 TCP监听端口关闭。
[Debug] 2022-10-17 20:40:10 TCP等待子协程关闭……
[Debug] 2022-10-17 20:40:15 127.0.0.1:57259 tcpServerReceive stop
[Debug] 2022-10-17 20:40:15 127.0.0.1:4568 tcpClientReceive stop
[Debug] 2022-10-17 20:40:15 127.0.0.1:4568 tcpClientReceive stop
[Debug] 2022-10-17 20:40:15 127.0.0.1:57258 tcpServerReceive stop
[Debug] 2022-10-17 20:40:15 TCPServer关闭。
*/
func TestTCPEcho(t *testing.T) {
	StartGuardian()
	defer StopGuardian()
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	var num, count int
	//50000
	loop := 50000
	stop := make(chan struct{})
	hs := func(msg []byte, send func([]byte) error) error {
		data := make([]byte, len(msg))
		copy(data, msg)
		send(data)
		return nil
	}
	s := NewTCPServer(":4568", nil, hs, logger)
	go s.Run()
	hc := func(msg []byte, send func([]byte) error) error {
		n := int(utils.BytesToInteger32[uint32](msg[6:10]))
		num++
		count += n
		if num == loop {
			time.Sleep(250 * time.Millisecond)
			close(stop)
		}
		return nil
	}
	c, err := TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, nil, hc, logger)
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
[Debug] 2022-10-17 20:52:13 TCP监听端口:4568
[Debug] 2022-10-17 20:52:13 TCP已初始化连接，等待客户端连接……
[Debug] 2022-10-17 20:52:13 TCP等待子协程关闭……
[Debug] 2022-10-17 20:52:13 TCP监听端口关闭。
[Debug] 2022-10-17 20:52:18 127.0.0.1:4568 tcpPing stop
[Debug] 2022-10-17 20:52:18 127.0.0.1:4568 tcpClientReceive stop
[Debug] 2022-10-17 20:52:18 127.0.0.1:60070 tcpServerReceive stop
[Debug] 2022-10-17 20:52:18 TCPServer关闭。
[Debug] 2022-10-17 20:52:19 1249975000 = 1249975000
*/
