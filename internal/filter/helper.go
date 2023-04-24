package filter

// dropByte 丢弃字符
//	drop 要丢弃的字符: []{起始位置(包含)，结束位置(不包含)}
func dropByte(src []byte, drop [][2]int) []byte {
	length := len(src)
	if length == 0 {
		return src
	}
	if len(drop) == 0 {
		return src
	}

	b1, cnt, copyFrom := make([]byte, length), 0, 0
	for _, arr := range drop {
		copyTo := arr[0]
		if n := copyTo - copyFrom; n > 0 {
			copy(b1[cnt:], src[copyFrom:copyTo])
			cnt += n
		}
		copyFrom = arr[1]
	}
	if n := length - copyFrom; n > 0 {
		copy(b1[cnt:], src[copyFrom:])
		cnt += n
	}
	return b1[:cnt]
}
