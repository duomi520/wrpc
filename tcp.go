package wrpc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duomi520/utils"
)

// TCPBufferSize 缓存大小
var TCPBufferSize = 4 * 1024

// TCPServer TCP服务
type TCPServer struct {
	snowFlakeID         *utils.SnowFlakeID
	tcpAddress          *net.TCPAddr
	tcpListener         *net.TCPListener
	ProtocolMagicNumber uint32
	tcpPort             string
	Handler             func([]byte, io.Writer) error
	closeOnce           sync.Once
	Logger              *slog.Logger
}

// NewTCPServer 新建
func NewTCPServer(snowFlakeID *utils.SnowFlakeID, port string, handler func([]byte, io.Writer) error, logger *slog.Logger) *TCPServer {
	var msg string
	if handler == nil {
		msg = "TCPServer Handler不为nil"
		logger.Error(msg)
		panic(msg)
	}
	tcpAddress, err := net.ResolveTCPAddr("tcp4", port)
	if err != nil {
		msg = fmt.Sprintf("net.ResolveTCPAddr失败:%s", err.Error())
		logger.Error(msg)
		panic(msg)
	}
	listener, err := net.ListenTCP("tcp", tcpAddress)
	if err != nil {
		msg = fmt.Sprintf("TCP监听端口失败:%s", err.Error())
		logger.Error(msg)
		panic(msg)
	}
	s := &TCPServer{
		snowFlakeID:         snowFlakeID,
		tcpAddress:          tcpAddress,
		tcpListener:         listener,
		ProtocolMagicNumber: defaultProtocolMagicNumber,
		tcpPort:             port,
		Handler:             handler,
		Logger:              logger,
	}
	return s
}

// Stop 关闭
func (s *TCPServer) Stop() {
	s.closeOnce.Do(func() {
		if err := s.tcpListener.Close(); err != nil {
			s.Logger.Error("TCPServer.Stop:TCP监听端口关闭失败", slog.String("error", err.Error()))
		} else {
			s.Logger.Debug("TCPServer.Stop:TCP监听端口关闭")
		}
	})
}

// Run 运行
func (s *TCPServer) Run() {
	var connMap sync.Map
	s.Logger.Debug("TCPServer.Run:TCP监听端口 " + s.tcpPort)
	s.Logger.Debug("TCPServer.Run:TCP已初始化连接,等待客户端连接……")
	tcpGroup := sync.WaitGroup{}
	//设置重试延迟时间
	var tempDelay time.Duration
	for {
		conn, err := s.tcpListener.AcceptTCP()
		if err != nil {
			if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
				s.Logger.Warn("TCPServer.Run:Temporary error when accepting new connections", slog.String("error", err.Error()))
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					//设置延迟时间,开始递增,避免无意义的重试
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
				s.Logger.Warn("TCPServer.Run:Permanent error when accepting new connections", slog.String("error", err.Error()))
			}
			break
		}
		//如果没有网络错误,下次网络错误的重试时间重新开始
		tempDelay = 0
		if err = conn.SetNoDelay(false); err != nil {
			s.Logger.Error("TCPServer.Run:TCP设定操作系统是否应该延迟数据包传递失败", slog.String("error", err.Error()))
		}
		//协议头
		pm := make([]byte, 4)
		if err := conn.SetReadDeadline(time.Now().Add(DefaultDeadlineDuration)); err != nil {
			s.Logger.Error("TCPServer.Run:读取协议头超时", slog.String("remoteAddr", conn.RemoteAddr().String()), slog.String("error", err.Error()))
			continue
		}
		if _, err := conn.Read(pm); err != nil {
			s.Logger.Error("TCPServer.Run:读取协议头失败", slog.String("remoteAddr", conn.RemoteAddr().String()), slog.String("error", err.Error()))
			continue
		}
		if binary.LittleEndian.Uint32(pm) != s.ProtocolMagicNumber {
			s.Logger.Warn("TCPServer.Run:无效的协议头", slog.String("remoteAddr", conn.RemoteAddr().String()))
			continue
		}
		//设置TCP保持长连接
		conn.SetKeepAlive(true)
		//设置TCP探测时间间隔时间为3分钟,如果客户端3分钟没有和服务端通信,则开始探测
		conn.SetKeepAlivePeriod(3 * time.Minute)
		conn.SetLinger(10)
		session := &TCPSession{
			conn:     conn,
			Handler:  s.Handler,
			stopFlag: 0,
			Logger:   s.Logger,
		}
		session.id, _ = s.snowFlakeID.NextID()
		connMap.Store(session.id, session)
		tcpGroup.Add(1)
		go func() {
			session.tcpServerReceive()
			tcpGroup.Done()
			connMap.Delete(session.id)
			session.close()
		}()
	}
	s.Logger.Debug("TCPServer.Run:TCPServer停止接受新连接,等待子协程关闭……")
	connMap.Range(func(_, value any) bool {
		value.(*TCPSession).close()
		return true
	})
	tcpGroup.Wait()
	s.Logger.Debug("TCPServer.Run:TCPServer关闭")
}

// TCPSession 会话
type TCPSession struct {
	id       int64
	conn     *net.TCPConn
	Handler  func([]byte, io.Writer) error
	stopChan chan struct{}
	// 1 - stop
	stopFlag  int32
	closeOnce sync.Once
	Logger    *slog.Logger
}

func (t *TCPSession) writeWithDeadline(d time.Duration, msg []byte) error {
	if err := t.conn.SetWriteDeadline(time.Now().Add(d)); err != nil {
		return err
	}
	if _, err := t.conn.Write(msg); err != nil {
		return err
	}
	return nil
}

// close 关闭
func (t *TCPSession) close() {
	t.closeOnce.Do(func() {
		atomic.StoreInt32(&t.stopFlag, 1)
		var err error
		err = t.conn.SetReadDeadline(time.Now())
		if err != nil {
			t.Logger.Warn("TCPSession.close(1):", slog.String("error", err.Error()))
		}
		err = t.conn.SetWriteDeadline(time.Now())
		if err != nil {
			t.Logger.Warn("TCPSession.close(2):", slog.String("error", err.Error()))
		}
		err = t.conn.Close()
		if err != nil {
			t.Logger.Warn("TCPSession.close(3):", slog.String("error", err.Error()))
		}
	})
}

func (ts *TCPSession) tcpServerReceive() {
	ts.Logger.Debug("TCPSession.tcpServerReceive:" + ts.conn.RemoteAddr().String() + " 开启")
	//IO读缓存
	buf := make([]byte, TCPBufferSize)
	//buf 写、读位置序号
	var w, r int
	var err error
	for {
		if err = ts.conn.SetReadDeadline(time.Now().Add(DefaultDeadlineDuration)); err != nil {
			goto end
		}
		n, err := ts.conn.Read(buf[w:])
		w += n
		if err != nil {
			goto end
		}
		//处理数据
		for w >= (r + 6) {
			tail := r + getLen(buf[r:])
			if tail > w {
				break
			}
			frame := buf[r:tail]
			switch getStatus(frame) {
			case utils.StatusPing16:
				r += 6
				_, err = ts.conn.Write(framePong)
				if err != nil {
					goto end
				}
			default:
				// 不能阻塞,是否创建协程由使用者决策
				if err := ts.Handler(frame, ts.conn); err != nil {
					ts.Logger.Error("TCPSession.tcpServerReceive(1):", slog.String("error", err.Error()))
					err = nil
				}
				r = tail
			}
		}
		if r < w {
			copy(buf[0:], buf[r:w])
			w -= r
		} else {
			w = 0
		}
		r = 0
	}
end:
	if err != nil && err != io.EOF {
		if !strings.Contains(err.Error(), "wsarecv: An existing connection was forcibly closed by the remote host.") && !strings.Contains(err.Error(), "use of closed network connection") {
			ts.Logger.Error("TCPSession.tcpServerReceive(2):", slog.String("remoteAddr", ts.conn.RemoteAddr().String()), slog.String("error", err.Error()))
		}
	}
	ts.Logger.Debug("TCPSession.tcpServerReceive:" + ts.conn.RemoteAddr().String() + " 关闭")
}

// TCPDial 连接
func TCPDial(url string, protocolMagicNumber uint32, handler func([]byte, io.Writer) error, logger *slog.Logger) (*TCPSession, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", url)
	if err != nil {
		return nil, fmt.Errorf("TCPDial:tcpAddr fail:%w", err)
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, fmt.Errorf("TCPDial:连接服务端失败:%w", err)
	}
	if conn.SetNoDelay(false) != nil {
		return nil, errors.New("TCPDial:设定操作系统是否应该延迟数据包传递失败")
	}
	session := &TCPSession{
		conn:     conn,
		Handler:  handler,
		stopChan: make(chan struct{}),
		stopFlag: 0,
		Logger:   logger,
	}
	//发送协议头
	pm := make([]byte, 4)
	binary.LittleEndian.PutUint32(pm, protocolMagicNumber)
	if err := session.writeWithDeadline(DefaultDeadlineDuration, pm); err != nil {
		return nil, fmt.Errorf("TCPDial:写入协议头失败:%w", err)
	} else {
		go func() {
			session.tcpClientReceive()
			close(session.stopChan)
			session.close()
		}()
		go session.tcpClientSend()
		return session, nil
	}
}

func (ts *TCPSession) tcpClientReceive() {
	ts.Logger.Debug("TCPSession.tcpClientReceive:" + ts.conn.RemoteAddr().String() + " 开启")
	//IO读缓存
	buf := make([]byte, TCPBufferSize)
	//buf 写、读位置序号
	var iw, ir int
	var err error
	for {
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
			length := int(binary.LittleEndian.Uint32(buf[ir : ir+4]))
			tail := ir + length
			if tail > iw {
				break
			}
			flag := binary.LittleEndian.Uint16(buf[ir+4 : ir+6])
			if flag == utils.StatusPong16 {
				ir += 6
			} else {
				if err := ts.Handler(buf[ir:tail], ts.conn); err != nil {
					ts.Logger.Error("TCPSession.tcpClientReceive(1):", slog.String("error", err.Error()))
					err = nil
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
			ts.Logger.Error("TCPSession.tcpClientReceive(2):", slog.String("remoteAddr", ts.conn.RemoteAddr().String()), slog.String("error", err.Error()))
		}
	}
	ts.Logger.Debug("TCPSession.tcpClientReceive:" + ts.conn.RemoteAddr().String() + " 关闭")
}

func (ts *TCPSession) tcpClientSend() {
	ts.Logger.Debug("TCPSession.tcpClientSend:" + ts.conn.RemoteAddr().String() + " 开启")
	timer := time.NewTimer(DefaultHeartbeatPeriodDuration)
	for {
		select {
		case <-timer.C:
			_, err := ts.conn.Write(framePing)
			if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				ts.Logger.Error("TCPSession.tcpClientSend:", slog.String("remoteAddr", ts.conn.RemoteAddr().String()), slog.String("error", err.Error()))
				goto end
			}
		case <-ts.stopChan:
			goto end
		}
	}
end:
	timer.Stop()
	ts.Logger.Debug("TCPSession.tcpClientSend:" + ts.conn.RemoteAddr().String() + " 关闭")
}

// http://xiaorui.cc/archives/6402
