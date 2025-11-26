package vector

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Float64SliceToBytes 将 []float64 转换为 []byte（小端序）
func Float64SliceToBytes(f []float64) ([]byte, error) {
	if f == nil {
		return nil, nil
	}
	buf := make([]byte, len(f)*8) // 每个 float64 占 8 字节
	for i, v := range f {
		bits := math.Float64bits(v)
		binary.LittleEndian.PutUint64(buf[i*8:], bits)
	}
	return buf, nil
}

// BytesToFloat64Slice 将 []byte 反序列化为 []float64
func BytesToFloat64Slice(b []byte) ([]float64, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if len(b)%8 != 0 {
		return nil, fmt.Errorf("invalid byte length for float64 slice: %d", len(b))
	}
	f := make([]float64, len(b)/8)
	for i := 0; i < len(f); i++ {
		bits := binary.LittleEndian.Uint64(b[i*8:])
		f[i] = math.Float64frombits(bits)
	}
	return f, nil
}

func TruncateToRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
