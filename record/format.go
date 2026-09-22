package record

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

// 录制文件格式(全部小端)：
//
//	文件头(默认128字节，HeaderSize字段声明实际大小)：
//	  0x00  6B  Magic "ESHREC"
//	  0x06  2B  Version(当前=1)
//	  0x08  4B  HeaderSize(含元数据区；向后兼容扩展)
//	  0x0C  4B  Flags(bit0=包含输入方向帧)
//	  0x10  8B  StartedAt(Unix 微秒，UTC)
//	  0x18  8B  DurationMicros(录制中为0，Close 时回写)
//	  0x20  44B 元数据 TLV 区: 重复 {1B Type, 2B Len, Len B Value}，余量补零
//
//	帧(循环至EOF)，帧头16字节：
//	  0x00  8B  DeltaMicros(相对 StartedAt 的微秒，单调递增)
//	  0x08  1B  Direction(0=out 远端→本地；1=in 本地→远端；2=event 事件)
//	  0x09  1B  Flags(bit0=secret，内容含密码，Dump 时打码)
//	  0x0A  2B  Reserved(0)
//	  0x0C  4B  PayloadLen
//	  0x10  NB  Payload(原始字节，未过滤未解码)

const (
	magic     = "ESHREC"
	version   = uint16(1)
	headerLen = 128
	frameLen  = 16

	metaAreaLen = headerLen - 0x20

	flagHasInput uint32 = 1 << 0
	frameSecret  byte   = 1 << 0
)

// Direction 帧的方向
type Direction uint8

const (
	DirOut   Direction = 0 // 远端→本地(shell 输出)
	DirIn    Direction = 1 // 本地→远端(写入的命令/应答)
	DirEvent Direction = 2 // 事件(连接关闭、错误等文本)
)

// 元数据 TLV 类型
const (
	metaTypeHost     byte = 1
	metaTypePort     byte = 2
	metaTypeProtocol byte = 3
	metaTypeUser     byte = 4
	metaTypeComment  byte = 5
)

// Meta 录制文件的元数据
type Meta struct {
	Host      string
	Port      int
	Protocol  string // "ssh" / "telnet" / "cmd"
	User      string
	Comment   string
	StartedAt time.Time
}

var errBadMagic = errors.New("record: bad magic, not an easyshell recording")
var errBadVersion = errors.New("record: unsupported recording version")

// writeHeader 写入文件头
func writeHeader(w io.Writer, meta Meta, flags uint32) error {
	buf := make([]byte, headerLen)
	copy(buf[0x00:], magic)
	binary.LittleEndian.PutUint16(buf[0x06:], version)
	binary.LittleEndian.PutUint32(buf[0x08:], headerLen)
	binary.LittleEndian.PutUint32(buf[0x0C:], flags)
	startedAt := meta.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	binary.LittleEndian.PutUint64(buf[0x10:], uint64(startedAt.UnixMicro()))
	// DurationMicros 在 Close 时回写

	// 元数据 TLV
	var tlvBuf bytes.Buffer
	putTLV := func(typ byte, val []byte) {
		if len(val) == 0 || tlvBuf.Len()+3+len(val) > metaAreaLen {
			return
		}
		tlvBuf.WriteByte(typ)
		var l [2]byte
		binary.LittleEndian.PutUint16(l[:], uint16(len(val)))
		tlvBuf.Write(l[:])
		tlvBuf.Write(val)
	}
	putTLV(metaTypeHost, []byte(meta.Host))
	if meta.Port > 0 {
		var p [4]byte
		binary.LittleEndian.PutUint32(p[:], uint32(meta.Port))
		putTLV(metaTypePort, p[:])
	}
	putTLV(metaTypeProtocol, []byte(meta.Protocol))
	putTLV(metaTypeUser, []byte(meta.User))
	putTLV(metaTypeComment, []byte(meta.Comment))
	copy(buf[0x20:], tlvBuf.Bytes())

	_, err := w.Write(buf)
	return err
}

// readHeader 读取并校验文件头，返回 startedAt、duration、flags
func readHeader(r io.Reader) (Meta, uint32, error) {
	buf := make([]byte, headerLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Meta{}, 0, err
	}
	if !bytes.Equal(buf[0x00:0x06], []byte(magic)) {
		return Meta{}, 0, errBadMagic
	}
	ver := binary.LittleEndian.Uint16(buf[0x06:])
	if ver != version {
		return Meta{}, 0, errBadVersion
	}

	var meta Meta
	meta.StartedAt = time.UnixMicro(int64(binary.LittleEndian.Uint64(buf[0x10:])))
	duration := int64(binary.LittleEndian.Uint64(buf[0x18:]))
	_ = duration // 由调用方按需读取(位置固定)

	// 元数据 TLV
	tlv := buf[0x20:]
	for i := 0; i+3 <= len(tlv); {
		typ := tlv[i]
		n := int(binary.LittleEndian.Uint16(tlv[i+1 : i+3]))
		if n < 0 || i+3+n > len(tlv) {
			break
		}
		val := tlv[i+3 : i+3+n]
		switch typ {
		case metaTypeHost:
			meta.Host = string(val)
		case metaTypePort:
			meta.Port = int(binary.LittleEndian.Uint32(val))
		case metaTypeProtocol:
			meta.Protocol = string(val)
		case metaTypeUser:
			meta.User = string(val)
		case metaTypeComment:
			meta.Comment = string(val)
		}
		i += 3 + n
	}
	flags := binary.LittleEndian.Uint32(buf[0x0C:])
	return meta, flags, nil
}

// writeFrame 写入一帧
func writeFrame(w io.Writer, deltaMicros int64, dir Direction, flags byte, payload []byte) error {
	head := make([]byte, frameLen)
	binary.LittleEndian.PutUint64(head[0x00:], uint64(deltaMicros))
	head[0x08] = byte(dir)
	head[0x09] = flags
	binary.LittleEndian.PutUint32(head[0x0C:], uint32(len(payload)))
	if _, err := w.Write(head); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := w.Write(payload)
		return err
	}
	return nil
}

// Frame 录制的帧
type Frame struct {
	// Delta 相对录制起始时间的偏移
	Delta time.Duration
	// Dir 帧方向
	Dir Direction
	// Secret 内容包含密码(Dump 时默认打码)
	Secret bool
	// Payload 原始字节内容
	Payload []byte
}

// Wall 返回帧的墙钟时间
func (f Frame) Wall(startedAt time.Time) time.Time {
	return startedAt.Add(f.Delta)
}

// readFrame 读取一帧；EOF 返回 io.EOF
func readFrame(r io.Reader) (Frame, error) {
	head := make([]byte, frameLen)
	if _, err := io.ReadFull(r, head); err != nil {
		return Frame{}, err
	}
	f := Frame{
		Delta:  time.Duration(int64(binary.LittleEndian.Uint64(head[0x00:]))),
		Dir:    Direction(head[0x08]),
		Secret: head[0x09]&frameSecret != 0,
	}
	n := binary.LittleEndian.Uint32(head[0x0C:])
	if n > 0 {
		f.Payload = make([]byte, n)
		if _, err := io.ReadFull(r, f.Payload); err != nil {
			return Frame{}, err
		}
	}
	return f, nil
}
