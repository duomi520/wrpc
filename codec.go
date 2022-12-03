package wrpc

import (
	"errors"
	"io"

	"github.com/duomi520/utils"
)

var framePing, framePong, frameGoaway, frameCtxCancelFunc []byte

func init() {
	framePing = make([]byte, 6)
	framePong = make([]byte, 6)
	frameGoaway = make([]byte, 6)
	frameCtxCancelFunc = make([]byte, 18)
	utils.CopyInteger32(framePing[0:4], uint32(6))
	utils.CopyInteger32(framePong[0:4], uint32(6))
	utils.CopyInteger32(frameGoaway[0:4], uint32(6))
	utils.CopyInteger32(frameCtxCancelFunc[0:4], uint32(18))
	utils.CopyInteger16(framePing[4:6], utils.StatusPing16)
	utils.CopyInteger16(framePong[4:6], utils.StatusPong16)
	utils.CopyInteger16(frameGoaway[4:6], utils.StatusGoaway16)
	utils.CopyInteger16(frameCtxCancelFunc[4:6], utils.StatusCtxCancelFunc16)
	utils.CopyInteger16(frameCtxCancelFunc[14:16], uint16(18))
	utils.CopyInteger16(frameCtxCancelFunc[16:18], uint16(18))
}

/*
+-------+-------+-------+-------+-------+-------+
|           Lenght(32)          |   Status(16)  |
+-------+-------+-------+-------+-------+-------+-------+-------+
|                          Seq (64)                             |
+-------+-------+-------+-------+-------+-------+-------+-------+
| ServiceMethodEnd(16)  |    MetadataEnd(16)    |				18
+-------+-------+-------+-------+-------+-------+-------+-------+
|                     ServiceMethod string                     ...
+-------+-------+-------+-------+-------+-------+-------+-------+
|                     Metadata map[any]any                     ...
+-------+-------+-------+-------+-------+-------+-------+-------+
|                       Payload (0...)                         ...
+-------+-------+-------+-------+-------+-------+-------+-------+
*/

//FrameMinLenght 长度
const FrameMinLenght int = 18

//Frame 帧
type Frame struct {
	Status        uint16
	Seq           int64
	ServiceMethod string
	Metadata      *utils.MetaDict
	Payload       any
}

//MarshalBinary 编码
func (f Frame) MarshalBinary(marshal func(any, io.Writer) error, buf *buffer) error {
	buf.reset()
	d := buf.getbuf()
	lenght := 0
	ServiceMethodEnd := 18 + uint16(len(f.ServiceMethod))
	MetadataEnd := ServiceMethodEnd
	//编码Metadata
	if f.Metadata != nil {
		var n int
		var e error
		n, e = f.Metadata.Encode(d[ServiceMethodEnd:])
		if e != nil {
			n, _ = f.Metadata.Encode(d[ServiceMethodEnd:])
		}
		MetadataEnd += uint16(n)
	}
	utils.CopyInteger32(d[0:4], uint32(lenght))
	utils.CopyInteger16(d[4:6], f.Status)
	utils.CopyInteger64(d[6:14], f.Seq)
	utils.CopyInteger16(d[14:16], ServiceMethodEnd)
	utils.CopyInteger16(d[16:18], MetadataEnd)
	copy(d[18:ServiceMethodEnd], utils.StringToBytes(f.ServiceMethod))
	//编码Payload
	buf.setValid(int(MetadataEnd))
	err := marshal(f.Payload, buf)
	if err != nil {
		return err
	}
	utils.CopyInteger32(d[0:4], uint32(buf.valid))
	return nil
}

//UnmarshalHeader 解码头部，Payload不解析，返会头长度及错误
func (f *Frame) UnmarshalHeader(data []byte) (int, error) {
	if len(data) < FrameMinLenght {
		return 0, errors.New("Frame.UnmarshalHeader：bytes is too short")
	}
	f.Status = utils.BytesToInteger16[uint16](data[4:6])
	f.Seq = utils.BytesToInteger64[int64](data[6:14])
	ServiceMethodEnd := utils.BytesToInteger16[uint16](data[14:16])
	MetadataEnd := utils.BytesToInteger16[uint16](data[16:18])
	f.ServiceMethod = string(data[18:ServiceMethodEnd])
	if ServiceMethodEnd != MetadataEnd {
		var m utils.MetaDict
		f.Metadata = &m
		f.Metadata.Decode(data[ServiceMethodEnd:MetadataEnd])
	}
	return int(MetadataEnd), nil
}

//GetPayload
func GetPayload(unmarshal func([]byte, any) error, data []byte) (obj any) {
	if len(data) < FrameMinLenght {
		return nil
	}
	l := utils.BytesToInteger16[uint16](data[16:18])
	unmarshal(data[l:], &obj)
	return obj
}

//GetStatus
func GetStatus(data []byte) uint16 {
	if len(data) < FrameMinLenght {
		return utils.StatusUnknown16
	}
	return utils.BytesToInteger16[uint16](data[4:6])
}

/*
+-------+-------+-------+-------+-------+-------+
|           Lenght(32)          |   Status(16)  |
+-------+-------+-------+-------+-------+-------+-------+-------+
|                       Payload (0...)                         ...
+-------+-------+-------+-------+-------+-------+-------+-------+
*/

//发送的[]byte前部需留 6 的空间
func HijackerSend(data []byte, send WriterFunc) error {
	utils.CopyInteger32(data[0:4], uint32(len(data)))
	utils.CopyInteger16(data[4:6], utils.StatusHijacker16)
	return send(data)
}

// https://www.jianshu.com/p/e57ca4fec26f  HTTP2 详解
