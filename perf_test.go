package dataset

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bigCSV 生成 10000 行 × 50 列的混合类型 CSV。
func bigCSV() string {
	var sb strings.Builder
	sb.Grow(MaxRows * MaxCols * 8)
	head := make([]string, MaxCols)
	for c := range head {
		head[c] = fmt.Sprintf("col%d", c)
	}
	sb.WriteString(strings.Join(head, ","))
	sb.WriteByte('\n')
	for r := 0; r < MaxRows; r++ {
		for c := 0; c < MaxCols; c++ {
			if c > 0 {
				sb.WriteByte(',')
			}
			switch c % 4 {
			case 0:
				fmt.Fprintf(&sb, "%d", r*MaxCols+c)
			case 1:
				fmt.Fprintf(&sb, "%d.%d", r, c)
			case 2:
				fmt.Fprintf(&sb, "%d/%d/2024", r%12+1, r%28+1)
			default:
				fmt.Fprintf(&sb, "text-%d-%d", r, c)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestPerfImportExport(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	st, err := Open(filepath.Join(t.TempDir(), "perf.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	csv := bigCSV()

	start := time.Now()
	ds, err := st.Import("big", []byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	importDur := time.Since(start)
	if ds.Rows != MaxRows || len(ds.Columns) != MaxCols {
		t.Fatalf("rows=%d cols=%d", ds.Rows, len(ds.Columns))
	}

	start = time.Now()
	out, err := st.Export(ds.ID)
	if err != nil {
		t.Fatal(err)
	}
	exportDur := time.Since(start)

	t.Logf("import: %v, export: %v", importDur, exportDur)
	if importDur > 3*time.Second {
		t.Fatalf("import too slow: %v", importDur)
	}
	if exportDur > 3*time.Second {
		t.Fatalf("export too slow: %v", exportDur)
	}
	// 上限规模也满足再导入一致。
	d2, err := st.Import("big2", out)
	if err != nil {
		t.Fatal(err)
	}
	assertSameDataset(t, st, ds.ID, d2.ID)
}

func TestPersistentStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := st.Import("t", []byte("a,b\n1,x\n2,y\n"))
	if err != nil {
		t.Fatal(err)
	}
	id := ds.ID
	st.Close()

	// 重新打开，数据仍在。
	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	s, err := st2.Summary(id)
	if err != nil {
		t.Fatal(err)
	}
	if s.Rows != 2 || s.Columns[0].Type != TypeInt || s.Columns[1].Type != TypeText {
		t.Fatalf("%+v", s)
	}
	c, err := st2.Cell(id, 1, 1)
	if err != nil || c.Value != "y" {
		t.Fatalf("cell: %+v %v", c, err)
	}
	// 行号越界。
	if _, err := st2.Row(id, 2); err == nil {
		t.Fatal("row out of range should error")
	}
}
