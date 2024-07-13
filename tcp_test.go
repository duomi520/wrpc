package wrpc

import (
	"encoding/binary"
	"io"
	"log/slog"

	//"net"

	"strconv"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

func TestTCPDial(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	h := func([]byte, io.Writer) error { return nil }
	s := NewTCPServer(utils.NewSnowFlakeID(0, SnowFlakeStartupTime), ":4567", h, slog.Default())
	go s.Run()
	c, err := TCPDial("127.0.0.1:4567", defaultProtocolMagicNumber, h, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	c.close()
	time.Sleep(time.Second)
	s.Stop()
	time.Sleep(time.Second)
}

/*
2024/07/13 20:32:09 DEBUG TCPServer.Run:TCP监听端口 :4567
2024/07/13 20:32:09 DEBUG TCPSession.tcpClientSend:127.0.0.1:4567 开启
2024/07/13 20:32:09 DEBUG TCPServer.Run:TCP已初始化连接,等待客户端连接……
2024/07/13 20:32:09 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4567 开启
2024/07/13 20:32:09 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51564 开启
2024/07/13 20:32:09 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4567 关闭
2024/07/13 20:32:09 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51564 关闭
2024/07/13 20:32:09 DEBUG TCPSession.tcpClientSend:127.0.0.1:4567 关闭
2024/07/13 20:32:10 DEBUG TCPServer.Stop:TCP监听端口关闭
2024/07/13 20:32:10 DEBUG TCPServer.Run:TCPServer停止接受新连接,等待子协程关闭……
2024/07/13 20:32:10 DEBUG TCPServer.Run:TCPServer关闭
*/

func TestTCPGracefulStop(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	h := func([]byte, io.Writer) error { return nil }
	s := NewTCPServer(utils.NewSnowFlakeID(1, SnowFlakeStartupTime), ":4568", h, slog.Default())
	go s.Run()
	_, err := TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, h, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	_, err = TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, h, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	s.Stop()
	time.Sleep(3 * time.Second)
}

/*
2024/07/13 20:32:33 DEBUG TCPSession.tcpClientSend:127.0.0.1:4568 开启
2024/07/13 20:32:33 DEBUG TCPServer.Run:TCP监听端口 :4568
2024/07/13 20:32:33 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4568 开启
2024/07/13 20:32:33 DEBUG TCPServer.Run:TCP已初始化连接,等待客户端连接……
2024/07/13 20:32:33 DEBUG TCPSession.tcpClientSend:127.0.0.1:4568 开启
2024/07/13 20:32:33 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4568 开启
2024/07/13 20:32:33 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51573 开启
2024/07/13 20:32:33 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51574 开启
2024/07/13 20:32:33 DEBUG TCPServer.Stop:TCP监听端口关闭
2024/07/13 20:32:33 DEBUG TCPServer.Run:TCPServer停止接受新连接,等待子协程关闭……
2024/07/13 20:32:33 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51573 关闭
2024/07/13 20:32:33 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4568 关闭
2024/07/13 20:32:33 DEBUG TCPSession.tcpClientSend:127.0.0.1:4568 关闭
2024/07/13 20:32:33 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4568 关闭
2024/07/13 20:32:33 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51574 关闭
2024/07/13 20:32:33 DEBUG TCPSession.tcpClientSend:127.0.0.1:4568 关闭
2024/07/13 20:32:33 DEBUG TCPServer.Run:TCPServer关闭
*/

func TestTCPEcho(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	var num, count int
	//50000
	loop := 50000
	stop := make(chan struct{})
	hs := func(msg []byte, w io.Writer) error {
		data := make([]byte, len(msg))
		copy(data, msg)
		_, err := w.Write(data)
		if err != nil {
			t.Fatal(err.Error())
		}
		return nil
	}
	s := NewTCPServer(utils.NewSnowFlakeID(1, SnowFlakeStartupTime), ":4568", hs, slog.Default())
	go s.Run()
	hc := func(msg []byte, w io.Writer) error {
		n := int(binary.LittleEndian.Uint32(msg[6:10]))
		num++
		count += n
		if num == loop {
			time.Sleep(250 * time.Millisecond)
			close(stop)
		}
		return nil
	}
	c, err := TCPDial("127.0.0.1:4568", defaultProtocolMagicNumber, hc, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 10)
	binary.LittleEndian.PutUint32(data[0:4], uint32(10))
	for i := 0; i < loop; i++ {
		binary.LittleEndian.PutUint32(data[6:10], uint32(i))
		buf := make([]byte, 10)
		copy(buf, data)
		c.writeWithDeadline(5*time.Second, buf)
	}
	<-stop
	time.Sleep(250 * time.Millisecond)
	s.Stop()
	time.Sleep(3 * time.Second)
	if count != (loop-1)*loop/2 {
		slog.Debug(strconv.Itoa(count) + " <> " + strconv.Itoa((loop-1)*loop/2))
		t.Fatal("计算结果错误")
	}
	slog.Debug(strconv.Itoa(count) + " = " + strconv.Itoa((loop-1)*loop/2))
}

/*
2024/07/13 20:32:51 DEBUG TCPServer.Run:TCP监听端口 :4568
2024/07/13 20:32:51 DEBUG TCPServer.Run:TCP已初始化连接,等待客户端连接……
2024/07/13 20:32:51 DEBUG TCPSession.tcpClientSend:127.0.0.1:4568 开启
2024/07/13 20:32:51 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4568 开启
2024/07/13 20:32:51 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51585 开启
2024/07/13 20:32:52 DEBUG TCPServer.Stop:TCP监听端口关闭
2024/07/13 20:32:52 DEBUG TCPServer.Run:TCPServer停止接受新连接,等待子协程关闭……
2024/07/13 20:32:52 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51585 关闭
2024/07/13 20:32:52 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4568 关闭
2024/07/13 20:32:52 DEBUG TCPServer.Run:TCPServer关闭
2024/07/13 20:32:52 DEBUG TCPSession.tcpClientSend:127.0.0.1:4568 关闭
2024/07/13 20:32:55 DEBUG 1249975000 = 1249975000
*/
