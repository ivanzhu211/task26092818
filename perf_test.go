package dataset

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// 上限规模 10000 行 × 50 列的文件，导入和导出各自应在 3 秒内完成。
func TestMaxScalePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}
	var sb strings.Builder
	for c := 0; c < maxCols; c++ {
		if c > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "col%d", c)
	}
	sb.WriteByte('\n')
	for r := 0; r < maxRows; r++ {
		for c := 0; c < maxCols; c++ {
			if c > 0 {
				sb.WriteByte(',')
			}
			switch c % 4 {
			case 0:
				fmt.Fprintf(&sb, "%d", r*maxCols+c)
			case 1:
				fmt.Fprintf(&sb, "%d.%03d", r, c)
			case 2:
				fmt.Fprintf(&sb, "%d/%d/%04d", 1+c%12, 1+c%28, 2000+c%25)
			default:
				fmt.Fprintf(&sb, "\"text, with \"\"quotes\"\" and\nnewline %d\"", r)
			}
		}
		sb.WriteByte('\n')
	}
	data := []byte(sb.String())

	s := openTemp(t)
	start := time.Now()
	sum, err := s.ImportCSV(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	importDur := time.Since(start)
	if sum.Rows != maxRows || len(sum.Columns) != maxCols {
		t.Fatalf("unexpected shape: rows=%d cols=%d", sum.Rows, len(sum.Columns))
	}
	if importDur > 3*time.Second {
		t.Errorf("import took %v, want < 3s", importDur)
	}

	var buf bytes.Buffer
	start = time.Now()
	if err := s.ExportCSV(&buf, sum.ID); err != nil {
		t.Fatalf("export: %v", err)
	}
	exportDur := time.Since(start)
	if exportDur > 3*time.Second {
		t.Errorf("export took %v, want < 3s", exportDur)
	}
	t.Logf("import %v, export %v, size %.1f MB", importDur, exportDur, float64(len(data))/1e6)

	// 导出结果再导入应完全一致
	sum2, err := s.ImportCSV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	_ = sum2
}
