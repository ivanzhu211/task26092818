package dataset

import (
	"bufio"
	"io"
	"strings"
)

// writeCSV 把表头和记录按 CSV 规则写出，行尾用 LF。
// 缺失值写成空字段，空字符串写成 ""，含逗号/引号/回车/换行的值加引号，
// 内部引号写成两个，保证导出的文件再导入后每个单元格都一致。
func writeCSV(w io.Writer, header []string, rows []record) error {
	bw := bufio.NewWriter(w)
	for i, name := range header {
		if i > 0 {
			bw.WriteByte(',')
		}
		writeField(bw, &name)
	}
	bw.WriteByte('\n')
	for _, rec := range rows {
		for i, f := range rec {
			if i > 0 {
				bw.WriteByte(',')
			}
			writeField(bw, f)
		}
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

func writeField(bw *bufio.Writer, f *string) {
	if f == nil {
		return // 缺失值：空字段
	}
	s := *f
	if s == "" {
		bw.WriteString(`""`) // 空字符串不是缺失值
		return
	}
	if !strings.ContainsAny(s, ",\"\n\r") {
		bw.WriteString(s)
		return
	}
	bw.WriteByte('"')
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			bw.WriteString(s[start:i])
			bw.WriteString(`""`)
			start = i + 1
		}
	}
	bw.WriteString(s[start:])
	bw.WriteByte('"')
}
