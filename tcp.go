package wrpc

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duomi520/utils"
)

//TCPBufferSize 缓存大小
var TCPBufferSize = 4 * 1024

//defaultProtocolMagicNumber 缺省协议头
const defaultProtocolMagicNumber uint32 = 2299

//TCPServer TCP服务
type TCPServer struct {
	tcpAddress          *net.TCPAddr
	tcpListener         *net.TCPListener
	ProtocolMagicNumber uint32
	tcpPort             string
	Handler             func(func([]byte) error, []byte) error
	// 1 - stop
	stop   int32
	once   sync.Once
	Logger utils.ILogger
}

//NewTCPServer 新建
func NewTCPServer(port string, h func(func([]byte) error, []byte) error, logger utils.ILogger) *TCPServer {
	if h == nil {
		logger.Fatal("TCP Handler不为nil")
		return nil
	}
	tcpAddress, err := net.ResolveTCPAddr("tcp4", port)
	if err != nil {
		logger.Fatal("ResolveTCPAddr失败: ", err.Error())
		return nil
	}
	listener, err := net.ListenTCP("tcp", tcpAddress)
	if err != nil {
		logger.Fatal("TCP监听端口失败:", err.Error())
		return nil
	}
	s := &TCPServer{
		tcpAddress:          tcpAddress,
		tcpListener:         listener,
		ProtocolMagicNumber: defaultProtocolMagicNumber,
		tcpPort:             port,
		Handler:             h,
		stop:                0,
		Logger:              logger,
	}
	return s
}

//Stop 关闭
func (s *TCPServer) Stop() {
	atomic.StoreInt32(&s.stop, 1)
	s.once.Do(func() {
		if err := s.tcpListener.Close(); err != nil {
			s.Logger.Error("TCP监听端口关闭失败: ", err.Error())
		} else {
			s.Logger.Debug("TCP监听端口关闭。")
		}
		time.Sleep(100 * time.Millisecond)
	})
}

//Run 运行
func (s *TCPServer) Run() {
	s.Logger.Debug("TCP监听端口", s.tcpPort)
	s.Logger.Debug("TCP已初始化连接，等待客户端连接……")
	tcpGroup := sync.WaitGroup{}
	//设置重试延迟时间
	var tempDelay time.Duration
	for {
		conn, err := s.tcpListener.AcceptTCP()
		if err != nil {
			if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
				s.Logger.Warn("Temporary error when accepting new connections: ", err.Error())
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					//设置延迟时间，开始递增，避免无意义的重试
					tempDelay *= 2
				}
				//限制了最大重试时间在1s
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			if !strings.Contains(err.Error(), "use of closed network connection") {
				s.Logger.Warn("Permanent error when accepting new connections: ", err.Error())
			}
			break
		}
		//如果没有网络错误，下次网络错误的重试时间重新开始
		tempDelay = 0
		if err = conn.SetNoDelay(false); err != nil {
			s.Logger.Error("TCP设定操作系统是否应该延迟数据包传递失败: " + err.Error())
		}
		//协议头
		pm := make([]byte, 4)
		if err := conn.SetReadDeadline(time.Now().Add(DefaultDeadlineDuration)); err != nil {
			s.Logger.Error("读取协议头超时: ", conn.RemoteAddr().Network(), err.Error())
			continue
		}
		if _, err := conn.Read(pm); err != nil {
			s.Logger.Error("读取协议头失败: ", conn.RemoteAddr().Network(), err.Error())
			continue
		}
		if utils.BytesToInteger32[uint32](pm) != s.ProtocolMagicNumber {
			s.Logger.Warn("无效的协议头: ", conn.RemoteAddr().Network())
			continue
		}
		//设置TCP保持长连接
		conn.SetKeepAlive(true)
		//设置TCP探测时间间隔时间为3分钟，如果客户端3分钟没有和服务端通信，则开始探测
		conn.SetKeepAlivePeriod(3 * time.Minute)
		conn.SetLinger(10)
		session := &TCPSession{
			conn:     conn,
			Handler:  s.Handler,
			stopSign: &s.stop,
			Logger:   s.Logger,
		}
		tcpGroup.Add(1)
		go func() {
			session.tcpServerReceive()
			tcpGroup.Done()
		}()
	}
	s.Logger.Debug("TCP等待子协程关闭……")
	tcpGroup.Wait()
	s.Logger.Debug("TCPServer关闭。")
	time.Sleep(100 * time.Millisecond)
}

//TCPSession 会话
type TCPSession struct {
	conn    *net.TCPConn
	Handler func(func([]byte) error, []byte) error
	// 1 - stop
	stopSign *int32
	Logger   utils.ILogger
}

func (ts *TCPSession) tcpClientReceive() {
	//IO读缓存
	buf := make([]byte, TCPBufferSize)
	//buf 写、读位置序号
	var iw, ir int
	var err error
	for {
		if atomic.LoadInt32(ts.stopSign) == 1 {
			goto end
		}
		if err = ts.conn.SetReadDeadline(time.Now().Add(DefaultDeadlineDuration)); err != nil {
			goto end
		}
		n, errX := ts.conn.Read(buf[iw:])
		iw += n
		if errX != nil {
			err = errX
			goto end
		}
		//处理数据
		for iw >= (ir + 6) {
			length := int(utils.BytesToInteger32[uint32](buf[ir : ir+4]))
			tail := ir + length
			if tail > iw {
				break
			}
			flag := utils.BytesToInteger16[uint16](buf[ir+4 : ir+6])
			if flag == utils.StatusGoaway16 {
				goto end
			}
			if flag == utils.StatusPong16 {
				ir += 6
			} else {
				if err := ts.Handler(ts.Send, buf[ir:tail]); err != nil {
					ts.Logger.Error("tcpClientReceive：" + err.Error())
				}
				ir = tail
			}
		}
		if ir < iw {
			copy(buf[0:], buf[ir:iw])
			iw -= ir
		} else {
			iw = 0
		}
		ir = 0
	}
end:
	if err != nil && err != io.EOF {
		if !strings.Contains(err.Error(), "wsarecv: An existing connection was forcibly closed by the remote host.") && !strings.Contains(err.Error(), "use of closed network connection") {
			ts.Logger.Error(ts.conn.RemoteAddr().String(), " tcpClientReceive：", err.Error())
		} else {
			ts.Logger.Debug(ts.conn.RemoteAddr().String(), " tcpClientReceive：", err.Error())
		}
	}
	atomic.StoreInt32(ts.stopSign, 1)
	ts.Logger.Debug(ts.conn.RemoteAddr().String(), " tcpClientReceive stop")
}

func (ts *TCPSession) tcpServerReceive() {
	//未处理的数据
	var unhandled []byte
	var err error
	for {
		if atomic.LoadInt32(ts.stopSign) == 1 {
			goto end
		}
		sl := getSlot()
		if len(unhandled) > 0 {
			copy(sl.buf[0:], unhandled)
		}
		//TODO 动态调整响应时间，提供批量处理速度，减少切换
		if err = ts.conn.SetReadDeadline(time.Now().Add(DefaultDeadlineDuration)); err != nil {
			sl.release()
			goto end
		}
		n, err := ts.conn.Read(sl.buf[len(unhandled):])
		if err != nil {
			sl.release()
			goto end
		}
		size := n + len(unhandled)
		sl.setLen(uint64(size))
		unhandled = unhandled[0:0]
		cursor := 0
		//处理数据
		for size >= (cursor + 6) {
			length := int(utils.BytesToInteger32[uint32](sl.buf[cursor : cursor+4]))
			tail := cursor + length
			if tail > size {
				break
			}
			flag := utils.BytesToInteger16[uint16](sl.buf[cursor+4 : cursor+6])
			if flag == utils.StatusGoaway16 {
				if err := ts.sendWithDeadline(500*time.Millisecond, frameGoaway); err != nil {
					sl.release()
					goto end
				}
				sl.release()
				goto end
			}
			if flag == utils.StatusPing16 {
				cursor += 6
				err = ts.Send(framePong)
				if err != nil {
					sl.release()
					goto end
				}
				sl.used(6)
			} else {
				var tasker task
				tasker.l, tasker.r, tasker.s, tasker.session = cursor, tail, sl, ts
				wgo(tasker.do)
				cursor = tail
			}
		}
		if cursor < size {
			unhandled = append(unhandled, sl.buf[cursor:size]...)
			sl.used(uint64(size - cursor))
		}
	}
end:
	if err != nil && err != io.EOF {
		if !strings.Contains(err.Error(), "wsarecv: An existing connection was forcibly closed by the remote host.") && !strings.Contains(err.Error(), "use of closed network connection") {
			ts.Logger.Error(ts.conn.RemoteAddr().String(), " tcpServerReceive: ", err.Error())
		} else {
			ts.Logger.Debug(ts.conn.RemoteAddr().String(), " tcpServerReceive: ", err.Error())
		}
	}
	if err = ts.conn.Close(); err != nil {
		ts.Logger.Error(err.Error())
	}
	ts.Logger.Debug(ts.conn.RemoteAddr().String(), " tcpServerReceive stop")
}

type task struct {
	l       int
	r       int
	s       *slot
	session *TCPSession
}

func (t task) do() {
	defer func() {
		if err := recover(); err != nil {
			const size = 65536
			buf := make([]byte, size)
			end := runtime.Stack(buf, false)
			if end > size {
				end = size
			}
			buf = buf[:end]
			t.session.Logger.Error(fmt.Sprintf("task.do： %s \n%s", err, buf))
		}
	}()
	if err := t.session.Handler(t.session.Send, t.s.buf[t.l:t.r]); err != nil {
		t.session.Logger.Error("task.do: " + err.Error())
	}
	t.s.used(uint64(t.r - t.l))
}

//TCPDial 连接
func TCPDial(url string, protocolMagicNumber uint32, h func(func([]byte) error, []byte) error, logger utils.ILogger) (*TCPSession, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", url)
	if err != nil {
		return nil, errors.New("TCPDial tcpAddr fail: " + err.Error())
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, errors.New("TCPDial 连接服务端失败: " + err.Error())
	}
	if conn.SetNoDelay(false) != nil {
		return nil, errors.New("TCPDial 设定操作系统是否应该延迟数据包传递失败: " + err.Error())
	}
	var sign int32
	c := &TCPSession{
		conn:     conn,
		Handler:  h,
		stopSign: &sign,
		Logger:   logger,
	}
	go c.tcpClientReceive()
	if err := defaultGuardian.AddJob(c.tcpPing); err != nil {
		return nil, errors.New("定时任务失败: " + err.Error())
	}
	//发送协议头
	pm := make([]byte, 4)
	utils.CopyInteger32(pm, protocolMagicNumber)
	if err := c.sendWithDeadline(DefaultDeadlineDuration, pm); err != nil {
		atomic.StoreInt32(&sign, 1)
		return nil, errors.New("写入协议头失败: " + err.Error())
	}
	return c, nil
}
func (ts *TCPSession) tcpPing() bool {
	var exit bool
	defer func() {
		if exit {
			if err := ts.sendWithDeadline(500*time.Millisecond, frameGoaway); err != nil {
				ts.Logger.Error(err.Error())
			}
			atomic.StoreInt32(ts.stopSign, 1)
			ts.Logger.Debug(ts.conn.RemoteAddr().String(), " tcpPing stop")
		}
	}()
	if atomic.LoadInt32(ts.stopSign) == 1 {
		exit = true
		return exit
	}
	if err := ts.sendWithDeadline(500*time.Millisecond, framePing); err != nil {
		exit = true
		return exit
	}
	return exit
}

//Send 发送
func (ts *TCPSession) Send(message []byte) error {
	return ts.sendWithDeadline(DefaultDeadlineDuration, message)
}

func (ts *TCPSession) sendWithDeadline(d time.Duration, message []byte) error {
	if err := ts.conn.SetWriteDeadline(time.Now().Add(d)); err != nil {
		return err
	}
	if _, err := ts.conn.Write(message); err != nil {
		return err
	}
	return nil
}

// http://xiaorui.cc/archives/6402
