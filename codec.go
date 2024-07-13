package wrpc

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/duomi520/utils"
)

var framePing, framePong, frameCtxCancelFunc []byte

func init() {
	framePing = make([]byte, 6)
	framePong = make([]byte, 6)
	frameCtxCancelFunc = make([]byte, 24)
	binary.LittleEndian.PutUint32(framePing[0:4], uint32(6))
	binary.LittleEndian.PutUint32(framePong[0:4], uint32(6))
	binary.LittleEndian.PutUint32(frameCtxCancelFunc[0:4], uint32(24))
	binary.LittleEndian.PutUint16(framePing[4:6], utils.StatusPing16)
	binary.LittleEndian.PutUint16(framePong[4:6], utils.StatusPong16)
	binary.LittleEndian.PutUint16(frameCtxCancelFunc[4:6], utils.StatusCtxCancelFunc16)
	binary.LittleEndian.PutUint16(frameCtxCancelFunc[22:24], uint16(24))
}
func FrameCtxCancelFunc(id int64) []byte {
	f := make([]byte, 24)
	copy(f, frameCtxCancelFunc)
	binary.LittleEndian.PutUint64(f[6:], uint64(id))
	return f
}

/*
+-------+-------+-------+-------+-------+-------+
|           Lenght(32)          |   Status(16)  |
+-------+-------+-------+-------+-------+-------+-------+-------+
|                          Seq int64                            |
+-------+-------+-------+-------+-------+-------+-------+-------+
|                       ServiceMethod uint64                    |
+-------+-------+-------+-------+-------+-------+-------+-------+
|MetadataEnd(16)|				   Metadata                    ...
+-------+-------+-------+-------+-------+-------+-------+-------+
|                       Payload (0...)                         ...
+-------+-------+-------+-------+-------+-------+-------+-------+
*/

// Frame 帧
type Frame struct {
	Status uint16
	Seq    int64
	Method uint64
}

func NewFrame(status uint16, seq int64, method string) (f Frame) {
	f.Status = status
	f.Seq = seq
	f.Method = utils.Hash64FNV1A(method)
	return f
}

func FrameEncode(f Frame, metadata utils.MetaDict[string], payload any, w io.Writer, encoder func(any, io.Writer) error) error {
	buf := utils.AllocBuffer()
	defer utils.FreeBuffer(buf)
	var MetadataEnd uint16 = 24
	//编码头部
	var b0 [24]byte
	binary.LittleEndian.PutUint16(b0[4:6], f.Status)
	binary.LittleEndian.PutUint64(b0[6:14], uint64(f.Seq))
	binary.LittleEndian.PutUint64(b0[14:22], f.Method)
	binary.LittleEndian.PutUint16(b0[22:], MetadataEnd)
	_, err := buf.Write(b0[:])
	if err != nil {
		return err
	}
	//编码Metadata
	if metadata.Len() != 0 {
		err = utils.MetaDictEncoder(metadata, buf)
		if err != nil {
			return err
		}
		binary.LittleEndian.PutUint16(buf.Bytes()[22:24], uint16(buf.Len()))
	}
	//编码Payload
	err = encoder(payload, buf)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(buf.Bytes()[0:4], uint32(buf.Len()))
	io.Copy(w, buf)
	return nil
}

// FrameUnmarshal 解码
func FrameUnmarshal(data []byte, unmarshal func([]byte, any) error) (Frame, utils.MetaDict[string], any, error) {
	if len(data) < 24 {
		return Frame{}, utils.MetaDict[string]{}, nil, errors.New("Frame.Get: bytes is too short")
	}
	var f Frame
	f.Status = getStatus(data)
	f.Seq = getSeq(data)
	f.Method = binary.LittleEndian.Uint64(data[14:22])
	metadataEnd := binary.LittleEndian.Uint16(data[22:24])
	var obj any
	err := unmarshal(data[metadataEnd:], &obj)
	if err != nil {
		return Frame{}, utils.MetaDict[string]{}, nil, err
	}
	if metadataEnd > 24 {
		return f, utils.MetaDictDecode(data[24:metadataEnd]), obj, nil
	}
	return f, utils.MetaDict[string]{}, obj, nil
}

// GetFrame 解码帧，返会帧及错误
func GetFrame(data []byte) (Frame, error) {
	if len(data) < 24 {
		return Frame{}, errors.New("Frame.GetFrame: bytes is too short")
	}
	var f Frame
	f.Status = getStatus(data)
	f.Seq = getSeq(data)
	f.Method = binary.LittleEndian.Uint64(data[14:22])
	return f, nil
}

// GetMetadata
func GetMetadata(data []byte) (utils.MetaDict[string], error) {
	if len(data) < 24 {
		return utils.MetaDict[string]{}, errors.New("Frame.GetMetadata: bytes is too short")
	}
	metadataEnd := binary.LittleEndian.Uint16(data[22:24])
	if metadataEnd > 24 {
		return utils.MetaDictDecode(data[24:metadataEnd]), nil
	}
	return utils.MetaDict[string]{}, nil
}

// GetPayload
func GetPayload(data []byte, unmarshal func([]byte, any) error) (any, error) {
	if len(data) < 24 {
		return nil, errors.New("Frame.GetPayload: bytes is too short")
	}
	metadataEnd := binary.LittleEndian.Uint16(data[22:24])
	var obj any
	err := unmarshal(data[metadataEnd:], &obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// splitMetaPayload
func splitMetaPayload(data []byte) (utils.MetaDict[string], []byte, error) {
	if len(data) < 24 {
		return utils.MetaDict[string]{}, nil, errors.New("Frame.splitMetaPayload: bytes is too short")
	}
	metadataEnd := binary.LittleEndian.Uint16(data[22:24])
	if metadataEnd < 24 {
		return utils.MetaDict[string]{}, nil, errors.New("Frame.splitMetaPayload: metadataEnd is too short")
	}
	return utils.MetaDictDecode(data[24:metadataEnd]), data[metadataEnd:], nil
}

// getStatus
func getStatus(data []byte) uint16 {
	return binary.LittleEndian.Uint16(data[4:6])
}

// getLen
func getLen(data []byte) int {
	return int(binary.LittleEndian.Uint32(data[:4]))
}

func getSeq(data []byte) int64 {
	return int64(binary.LittleEndian.Uint64(data[6:14]))
}

// https://www.jianshu.com/p/e57ca4fec26f  HTTP2 详解
