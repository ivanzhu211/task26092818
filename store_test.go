package dataset

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestImportSummaryAndCell(t *testing.T) {
	s := openTemp(t)
	csv := "name,age,score,joined,zip,note\n" +
		"Alice,30,91.5,1/5/2023,02139,\"hello, world\"\n" +
		"Bob,,88.0,2/29/2020,02140,\"\"\n" +
		"Carol,25,,12/31/1999,10001,\"multi\nline\"\n"
	sum, err := s.ImportCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if sum.Rows != 3 || len(sum.Columns) != 6 {
		t.Fatalf("unexpected summary shape: %+v", sum)
	}
	wantTypes := []ColType{ColText, ColInt, ColFloat, ColDate, ColText, ColText}
	wantMissing := []int{0, 1, 1, 0, 0, 0}
	for i, c := range sum.Columns {
		if c.Type != wantTypes[i] || c.Missing != wantMissing[i] {
			t.Errorf("col %d (%s): got type=%v missing=%d", i, c.Name, c.Type, c.Missing)
		}
	}

	// 按行号查看单元格
	v, err := s.Cell(sum.ID, 0, 0)
	if err != nil || v.Str != "Alice" {
		t.Errorf("cell(0,0): got %+v, err=%v", v, err)
	}
	v, _ = s.Cell(sum.ID, 0, 1)
	if v.Int != 30 {
		t.Errorf("cell(0,1): got %+v", v)
	}
	v, _ = s.Cell(sum.ID, 0, 2)
	if v.Float != 91.5 {
		t.Errorf("cell(0,2): got %+v", v)
	}
	v, _ = s.Cell(sum.ID, 1, 1)
	if !v.Null {
		t.Errorf("cell(1,1) should be missing, got %+v", v)
	}
	v, _ = s.Cell(sum.ID, 1, 5)
	if v.Null || v.Str != "" {
		t.Errorf("cell(1,5) should be empty string, got %+v", v)
	}
	v, _ = s.Cell(sum.ID, 2, 5)
	if v.Str != "multi\nline" {
		t.Errorf("cell(2,5): got %q", v.Str)
	}
	v, _ = s.Cell(sum.ID, 0, 4)
	if v.Str != "02139" { // 邮编保留前导零
		t.Errorf("cell(0,4): got %q", v.Str)
	}
}

func TestRejectedImportLeavesNothing(t *testing.T) {
	s := openTemp(t)
	good, err := s.ImportCSV(strings.NewReader("a\n1\n"))
	if err != nil {
		t.Fatalf("import good: %v", err)
	}
	// 第二份文件格式错误，整份拒收
	_, err = s.ImportCSV(strings.NewReader("a,b\n1,\"unclosed\n"))
	if err == nil {
		t.Fatal("expected import to fail")
	}
	var ierr *ImportError
	if e, ok := err.(*ImportError); !ok || e.Category != ErrBadQuote {
		t.Fatalf("expected ImportError bad_quote, got %v (%T)", err, ierr)
	}
	// 不能留下半个数据集：库里只有第一份
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM datasets`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 dataset, got %d", n)
	}
	sum, err := s.SummaryOf(good.ID)
	if err != nil || sum.Rows != 1 {
		t.Fatalf("good dataset damaged: %+v, err=%v", sum, err)
	}
}

func TestExportRoundTrip(t *testing.T) {
	s := openTemp(t)
	csv := "text,int,float,date,mixed\n" +
		"\"a,b\",1,1.5,1/5/2023,x\n" +
		"\"say \"\"hi\"\"\",-2,2.0E+10,2/29/2020,1\n" +
		"\"\",0,1.5e-3,12/31/1999,\n" +
		"\" lead and trail \",007,0.5,1/1/2000,1/2/2023\n"
	sum1, err := s.ImportCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	var buf bytes.Buffer
	if err := s.ExportCSV(&buf, sum1.ID); err != nil {
		t.Fatalf("export: %v", err)
	}
	sum2, err := s.ImportCSV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-import exported csv: %v\nexported:\n%s", err, buf.String())
	}
	assertSameDataset(t, s, sum1.ID, sum2.ID)
}

// assertSameDataset 校验两个数据集完全一致：列名、列类型、行数、每个单元格。
func assertSameDataset(t *testing.T, s *Store, id1, id2 int64) {
	t.Helper()
	sum1, err := s.SummaryOf(id1)
	if err != nil {
		t.Fatal(err)
	}
	sum2, err := s.SummaryOf(id2)
	if err != nil {
		t.Fatal(err)
	}
	if sum1.Rows != sum2.Rows || len(sum1.Columns) != len(sum2.Columns) {
		t.Fatalf("shape differs: %+v vs %+v", sum1, sum2)
	}
	for i := range sum1.Columns {
		a, b := sum1.Columns[i], sum2.Columns[i]
		if a.Name != b.Name || a.Type != b.Type || a.Missing != b.Missing {
			t.Fatalf("column %d differs: %+v vs %+v", i, a, b)
		}
	}
	for r := 0; r < sum1.Rows; r++ {
		for c := range sum1.Columns {
			v1, err := s.Cell(id1, r, c)
			if err != nil {
				t.Fatal(err)
			}
			v2, err := s.Cell(id2, r, c)
			if err != nil {
				t.Fatal(err)
			}
			if v1 != v2 {
				t.Fatalf("cell(%d,%d) differs: %+v vs %+v", r, c, v1, v2)
			}
		}
	}
	// 导出字节也应一致
	var b1, b2 bytes.Buffer
	if err := s.ExportCSV(&b1, id1); err != nil {
		t.Fatal(err)
	}
	if err := s.ExportCSV(&b2, id2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
		t.Fatalf("exported bytes differ:\n%s\n---\n%s", b1.String(), b2.String())
	}
}
