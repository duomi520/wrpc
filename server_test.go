package wrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/duomi520/utils"
	"log"
	"strings"
	"testing"
	"time"
)

type Args struct {
	A, B int
}
type Quotient struct {
	Quo, Rem int
}
type Arith int

func (t *Arith) Add100(_ context.Context, args int) (int, error) {
	return args + 100, nil
}
func (t *Arith) Double(_ context.Context, args []int) ([]int, error) {
	var reply []int
	for i := range args {
		reply = append(reply, args[i]*2)
	}
	return reply, nil
}

func (t *Arith) Multiply(_ context.Context, args Args) (int, error) {
	return args.A * args.B, nil
}
func (t *Arith) Divide(_ context.Context, args Args) (Quotient, error) {
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
		reply, err := r.functionCall(v.key, nil, body)
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
}

/*
2022/01/26 18:52:54  0 | 106 = 106 |
2022/01/26 18:52:54  1 | [8 10 12 14 16] = [8 10 12 14 16] |
2022/01/26 18:52:54  2 | 56 = 56 |
2022/01/26 18:52:54  3 | {0 7} = {0 7} |
2022/01/26 18:52:54  4 | <nil> = {0 0} | divide by zero
*/

func TestRPC(t *testing.T) {
	logger, _ := utils.NewWLogger(utils.DebugLevel, "")
	defer logger.Close()
	o := NewOptions(WithLogger(logger))
	r := NewService(o)
	arith := new(Arith)
	if err := r.RegisterRPC("计算", arith); err != nil {
		t.Fatal(err)
	}
	ctx, ctxExitFunc := context.WithCancel(context.Background())
	s := NewTCPServer(ctx, ":4567", r.serveRequest, logger)
	go s.Run()
	rc1 := rpcResponseGet()
	rc2 := rpcResponseGet()
	rc3 := rpcResponseGet()
	defer func() {
		rpcResponsePut(rc1)
		rpcResponsePut(rc2)
		rpcResponsePut(rc3)
	}()
	c, err := NewTCPClient(context.TODO(), "127.0.0.1:4567", o)
	if err != nil {
		t.Fatal(err)
	}
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
	f1 := Frame{utils.StatusRequest16, 1, "计算.Multiply", nil, Args{7, 8}}
	buf, _ := f1.MarshalBinary(json.Marshal, makeBytes)
	if err := c.send(buf); err != nil {
		t.Fatal(err)
	}
	f2 := Frame{utils.StatusRequest16, 2, "计算.Divide", nil, Args{7, 8}}
	buf, _ = f2.MarshalBinary(json.Marshal, makeBytes)
	if err := c.send(buf); err != nil {
		t.Fatal(err)
	}
	f3 := Frame{utils.StatusRequest16, 3, "计算.Divide", nil, Args{9, 0}}
	buf, _ = f3.MarshalBinary(json.Marshal, makeBytes)
	if err := c.send(buf); err != nil {
		t.Fatal(err)
	}
	<-rc1.Done
	<-rc2.Done
	<-rc3.Done
	time.Sleep(250 * time.Millisecond)
	fmt.Println(reply1, reply2, rc3.Error)
	ctxExitFunc()
	time.Sleep(5 * time.Second)
}

/*
[Debug] 2022-01-27 13:45:52 TCP监听端口:4567
[Debug] 2022-01-27 13:45:52 TCP已初始化连接，等待客户端连接……
56 {0 7} divide by zero
[Debug] 2022-01-27 13:45:52 TCP监听端口关闭。
[Debug] 2022-01-27 13:45:52 TCP等待子协程关闭……
[Debug] 2022-01-27 13:45:56 127.0.0.1:50847 tcpReceive stop
[Debug] 2022-01-27 13:45:56 TCPServer关闭。
[Debug] 2022-01-27 13:45:56 127.0.0.1:4567 tcpSend stop
[Debug] 2022-01-27 13:45:56 127.0.0.1:4567 tcpReceive stop
*/
