package record

import (
	"bytes"
	"io"
	"os"
	"time"
)

// Player 录制回放器
type Player struct {
	meta     Meta
	duration time.Duration
	frames   []Frame
	file     *os.File
}

// Open 打开录制文件并载入全部帧
func Open(path string) (*Player, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	meta, _, err := readHeader(f)
	if err != nil {
		return nil, err
	}

	var frames []Frame
	for {
		frame, err := readFrame(f)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}

	// 时长：重新读取(Seek 不影响已读内容)
	duration, _ := durationAt(f)

	return &Player{meta: meta, duration: duration, frames: frames, file: f}, nil
}

// OpenBytes 从字节内容载入录制
func OpenBytes(data []byte) (*Player, error) {
	r := bytes.NewReader(data)
	meta, _, err := readHeader(r)
	if err != nil {
		return nil, err
	}

	var frames []Frame
	for {
		frame, err := readFrame(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
	duration, _ := durationAt(bytes.NewReader(data))
	return &Player{meta: meta, duration: duration, frames: frames}, nil
}

// Meta 返回录制元数据
func (p *Player) Meta() Meta {
	return p.meta
}

// Duration 返回录制总时长(Close 前录制中的文件为 0)
func (p *Player) Duration() time.Duration {
	return p.duration
}

// Frames 返回全部帧
func (p *Player) Frames() []Frame {
	return p.frames
}

// Close 关闭文件
func (p *Player) Close() error {
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}
