package wrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strconv"
	"strings"

	"sync"
	"testing"
	"time"

	"github.com/duomi520/utils"
)

type Args struct {
	A, B int
}
type Quotient struct {
	Quo, Rem int
}
type Arith int

func (t *Arith) Add100(_ *RPCContext, args int) (int, error) {
	return args + 100, nil
}
func (t *Arith) Double(_ *RPCContext, args []int) ([]int, error) {
	var reply []int
	for i := range args {
		reply = append(reply, args[i]*2)
	}
	return reply, nil
}

func (t *Arith) Multiply(_ *RPCContext, args Args) (int, error) {
	return args.A * args.B, nil
}
func (t *Arith) Divide(_ *RPCContext, args Args) (Quotient, error) {
	var quo Quotient
	if args.B == 0 {
		return quo, errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return quo, nil
}

func TestFunctionCall(t *testing.T) {
	type table struct {
		key    string
		args   any
		expect any
		err    string
	}
	var tests = []table{
		{"计算.Add100", 6, 106, ""},
		{"计算.Double", []int{4, 5, 6, 7, 8}, []int{8, 10, 12, 14, 16}, ""},
		{"计算.Multiply", Args{7, 8}, 56, ""},
		{"计算.Divide", Args{7, 8}, Quotient{0, 7}, ""},
		{"计算.Divide", Args{9, 0}, Quotient{0, 0}, "divide by zero"},
	}
	o := NewOptions()
	r := NewService(o)
	arith := new(Arith)
	if err := r.RegisterRPC("计算", arith); err != nil {
		t.Log(err.Error())
	}
	for index, v := range tests {
		body, _ := json.Marshal(&v.args)
		m := r.methodMap[utils.Hash64FNV1A(v.key)]
		reply, err := r.functionCall(m, utils.MetaDict[string]{}, body)
		if err != nil {
			if !strings.EqualFold(err.Error(), v.err) {
				t.Fatalf("%2d | %s | %s \n", index, err.Error(), v.err)
			}
		} else {
			r1, _ := json.Marshal(reply)
			r2, _ := json.Marshal(v.expect)
			if !bytes.EqualFold(r1, r2) {
				t.Fatalf("%2d | %v | %v \n", index, reply, v.expect)
			}
		}
		log.Printf("%2d | %v = %v | %v\n", index, reply, v.expect, v.err)
	}
	g, err := directCall(context.TODO(), r.methodMap[utils.Hash64FNV1A("计算.Add100")], 8)
	if err != nil {
		t.Fatal(err.Error())
	}
	log.Println(g)
}

/*
2024/07/13 20:30:14  0 | 106 = 106 |
2024/07/13 20:30:14  1 | [8 10 12 14 16] = [8 10 12 14 16] |
2024/07/13 20:30:14  2 | 56 = 56 |
2024/07/13 20:30:14  3 | {0 7} = {0 7} |
2024/07/13 20:30:14  4 | <nil> = {0 0} | divide by zero
2024/07/13 20:30:14 108
*/

func TestRPC(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	o := NewOptions(WithLogger(slog.Default()))
	r := NewService(o)
	arith := new(Arith)
	if err := r.RegisterRPC("计算", arith); err != nil {
		t.Fatal(err)
	}
	s := NewTCPServer(utils.NewSnowFlakeID(0, SnowFlakeStartupTime), ":4567", r.serveRequest, slog.Default())
	go s.Run()
	rc1 := AllocRPCResponse()
	rc2 := AllocRPCResponse()
	rc3 := AllocRPCResponse()
	defer func() {
		FreeRPCResponse(rc1)
		FreeRPCResponse(rc2)
		FreeRPCResponse(rc3)
	}()
	c, err := NewTCPClient("127.0.0.1:4567", NewOptions(WithLogger(slog.Default())))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var reply1 int
	rc1.reply = &reply1
	var reply2, reply3 Quotient
	rc2.reply = &reply2
	rc3.reply = &reply3
	rc1.id, rc2.id, rc3.id = 1, 2, 3
	rc1.client, rc2.client, rc3.client = c, c, c
	c.callMap.Store(int64(1), rc1)
	c.callMap.Store(int64(2), rc2)
	c.callMap.Store(int64(3), rc3)
	f1 := NewFrame(utils.StatusRequest16, 1, "计算.Multiply")
	err = FrameEncode(f1, utils.MetaDict[string]{}, Args{7, 8}, c.w, c.Encoder)
	if err != nil {
		t.Fatal(err)
	}
	f2 := NewFrame(utils.StatusRequest16, 2, "计算.Divide")
	err = FrameEncode(f2, utils.MetaDict[string]{}, Args{7, 8}, c.w, c.Encoder)
	if err != nil {
		t.Fatal(err)
	}
	f3 := NewFrame(utils.StatusRequest16, 3, "计算.Divide")
	err = FrameEncode(f3, utils.MetaDict[string]{}, Args{9, 0}, c.w, c.Encoder)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		<-rc1.Done
		wg.Done()
	}()
	go func() {
		<-rc2.Done
		wg.Done()
	}()
	go func() {
		<-rc3.Done
		wg.Done()
	}()
	wg.Wait()
	fmt.Println(reply1, reply2, rc3.Error)
	s.Stop()
	time.Sleep(time.Second)
}

/*
2024/07/13 20:30:30 DEBUG TCPServer.Run:TCP监听端口 :4567
2024/07/13 20:30:30 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4567 开启
2024/07/13 20:30:30 DEBUG TCPSession.tcpClientSend:127.0.0.1:4567 开启
2024/07/13 20:30:30 DEBUG TCPServer.Run:TCP已初始化连接,等待客户端连接……
2024/07/13 20:30:30 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51533 开启
56 {0 7} divide by zero
2024/07/13 20:30:30 DEBUG TCPServer.Stop:TCP监听端口关闭
2024/07/13 20:30:30 DEBUG TCPServer.Run:TCPServer停止接受新连接,等待子协程关闭……
2024/07/13 20:30:30 DEBUG TCPSession.tcpServerReceive:127.0.0.1:51533 关闭
2024/07/13 20:30:30 DEBUG TCPSession.tcpClientReceive:127.0.0.1:4567 关闭
2024/07/13 20:30:30 DEBUG TCPServer.Run:TCPServer关闭
2024/07/13 20:30:30 DEBUG TCPSession.tcpClientSend:127.0.0.1:4567 关闭
*/

func TestServerHook(t *testing.T) {
	h := func(proto func([]byte, io.Writer) error) func([]byte, io.Writer) error {
		return func(req []byte, rw io.Writer) error {
			log.Println("计算前")
			err := proto(req, rw)
			log.Println("计算后")
			return err
		}
	}
	slog.SetLogLoggerLevel(slog.LevelError)
	o := NewOptions(WithLogger(slog.Default()), WithWarpHandler(h))
	r := NewService(o)
	arith := new(Arith)
	if err := r.RegisterRPC("计算", arith); err != nil {
		t.Fatal(err)
	}
	err := r.TCPServer(":4567")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewTCPClient("127.0.0.1:4567", NewOptions(WithLogger(slog.Default())))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	reply := 0
	err = c.Call(context.TODO(), "计算.Add100", 8, &reply)
	if err != nil {
		t.Fatal(err)
	}
	if reply != 108 {
		t.Fatal("不等于108,返回值为:" + strconv.Itoa(reply))
	}
	r.Stop()
}

/*
2024/07/13 20:30:52 计算前
2024/07/13 20:30:53 计算后
*/
