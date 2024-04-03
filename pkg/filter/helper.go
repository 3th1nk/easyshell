package filter

// dropBytes 删除一段字符，并返回删除字符数量
//
//	s 要删除字符的源，会被修改
func dropBytes(s []byte, start int, end int) int {
	copy(s[start:], s[end:])
	return end - start
}

// dropMultiBytes 删除多段字符
//
//	s 要删除字符的源，会被修改
//	dropArr 要删除的字符: []{起始位置(包含)，结束位置(不包含)}
func dropMultiBytes(s []byte, dropArr [][2]int) []byte {
	srcLen := len(s)
	if srcLen == 0 || len(dropArr) == 0 {
		return s
	}

	var i, cnt int // i 表示待删除区间的左边界，cnt表示最终剩余字节数
	for _, v := range dropArr {
		// 当 dropArr 中的元素不合法时，跳过该元素
		if v[1] > srcLen || v[0] >= v[1] {
			continue
		}
		// 将需要保留的字节拷贝到前面的位置上
		copy(s[cnt:], s[i:v[0]])
		cnt += v[0] - i
		i = v[1] // 更新左边界
	}

	// 将最后一个待保留区间后面的字节拷贝到前面的位置上
	copy(s[cnt:], s[i:])
	cnt += srcLen - i

	return s[:cnt]
}
