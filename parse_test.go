package dataset

import (
	"strings"
	"testing"
)

// importOK 断言导入成功，返回摘要。
func importOK(t *testing.T, csv string) *Dataset {
	t.Helper()
	st, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ds, err := st.Import("t", []byte(csv))
	if err != nil {
		t.Fatalf("import should succeed: %v\ncsv: %q", err, csv)
	}
	return ds
}

// importErr 断言导入失败，且错误类别与记录序号符合预期。
func importErr(t *testing.T, csv string, cat ErrCategory, record int) {
	t.Helper()
	st, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, err = st.Import("t", []byte(csv))
	if err == nil {
		t.Fatalf("import should fail (%s)\ncsv: %q", cat, csv)
	}
	ie, ok := err.(*ImportError)
	if !ok {
		t.Fatalf("error should be *ImportError, got %T: %v", err, err)
	}
	if ie.Category != cat || ie.Record != record {
		t.Fatalf("want (%s, %d), got (%s, %d): %v\ncsv: %q",
			cat, record, ie.Category, ie.Record, ie, csv)
	}
	// 整份拒收：不能留下半个数据集。
	if _, serr := st.Summary(1); serr == nil {
		t.Fatalf("rejected import must not leave a dataset behind\ncsv: %q", csv)
	}
}

func TestBasicImport(t *testing.T) {
	ds := importOK(t, "name,age\nalice,30\nbob,25\n")
	if ds.Rows != 2 || len(ds.Columns) != 2 {
		t.Fatalf("bad summary: %+v", ds)
	}
	if ds.Columns[0].Name != "name" || ds.Columns[0].Type != TypeText {
		t.Fatalf("col0: %+v", ds.Columns[0])
	}
	if ds.Columns[1].Type != TypeInt || ds.Columns[1].Missing != 0 {
		t.Fatalf("col1: %+v", ds.Columns[1])
	}
}

func TestBOMAccepted(t *testing.T) {
	ds := importOK(t, "\xEF\xBB\xBFa,b\n1,2\n")
	if ds.Columns[0].Name != "a" {
		t.Fatalf("BOM not stripped: %+v", ds.Columns[0])
	}
}

func TestMixedLineEndings(t *testing.T) {
	ds := importOK(t, "a\r\nb\nc\r\n")
	if ds.Rows != 2 {
		t.Fatalf("rows = %d", ds.Rows)
	}
}

func TestTrailingNewlineNoExtraRow(t *testing.T) {
	ds := importOK(t, "a\n1\n")
	if ds.Rows != 1 {
		t.Fatalf("rows = %d", ds.Rows)
	}
	// 没有末尾换行也一样。
	ds = importOK(t, "a\n1")
	if ds.Rows != 1 {
		t.Fatalf("rows = %d", ds.Rows)
	}
}

func TestEmptyLineIsARow(t *testing.T) {
	// 单列数据集中，中间的空行算一条（缺失的）记录。
	ds := importOK(t, "a\n1\n\n2\n")
	if ds.Rows != 3 || ds.Columns[0].Missing != 1 {
		t.Fatalf("rows=%d missing=%d", ds.Rows, ds.Columns[0].Missing)
	}
	// 多列时，空行列数与表头不符，拒收。
	importErr(t, "a,b\n1,2\n\n3,4\n", CatShape, 2)
}

func TestQuotedCells(t *testing.T) {
	ds := importOK(t, "txt\n\"a,b\"\n")
	st, _ := Open(":memory:")
	defer st.Close()
	d2, _ := st.Import("t", []byte("txt\n\"he said \"\"hi\"\"\"\n\"line1\r\nline2\nline3\"\n"))
	c0, _ := st.Cell(d2.ID, 0, 0)
	if c0.Value != `he said "hi"` {
		t.Fatalf("quote unescape: %q", c0.Value)
	}
	c1, _ := st.Cell(d2.ID, 1, 0)
	if c1.Value != "line1\r\nline2\nline3" {
		t.Fatalf("quoted newlines must be preserved: %q", c1.Value)
	}
	_ = ds
}

func TestQuotedEmptyStringIsNotMissing(t *testing.T) {
	st, _ := Open(":memory:")
	defer st.Close()
	ds, err := st.Import("t", []byte("a,b\n,\"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if ds.Columns[0].Missing != 1 || ds.Columns[1].Missing != 0 {
		t.Fatalf("missing counts: %+v", ds.Columns)
	}
	c, _ := st.Cell(ds.ID, 0, 1)
	if c.Null || c.Value != "" {
		t.Fatalf("quoted empty string: %+v", c)
	}
	// 空串是文本值，列应为文本列。
	if ds.Columns[1].Type != TypeText {
		t.Fatalf("col type: %v", ds.Columns[1].Type)
	}
}

func TestQuoteSyntaxErrors(t *testing.T) {
	// 未加引号的单元格里出现引号。
	importErr(t, "a\nab\"cd\n", CatSyntax, 1)
	// 闭合引号后紧跟其他字符。
	importErr(t, "a\n\"ab\"cd\n", CatSyntax, 1)
	// 引号到文件结束未闭合。
	importErr(t, "a\n\"abc\n", CatSyntax, 1)
	importErr(t, "a\n\"abc", CatSyntax, 1)
	// 单独的 CR。
	importErr(t, "a\r1\n", CatSyntax, 0)
	// 错误记录序号：第二条数据记录。
	importErr(t, "a\n1\n2\nx\"y\n", CatSyntax, 3)
}

func TestInvalidUTF8(t *testing.T) {
	importErr(t, "a\n\xff\xfe\n", CatEncoding, 1)
	importErr(t, "a\n\"ok\xff\"\n", CatEncoding, 1)
	importErr(t, "\xffa\n", CatEncoding, 0)
}

func TestHeaderRules(t *testing.T) {
	// 空文件：没有表头。
	importErr(t, "", CatHeader, 0)
	// 列名为空（未加引号的空单元格）。
	importErr(t, "a,,c\n1,2,3\n", CatHeader, 0)
	// 列名为空（带引号的空串）。
	importErr(t, "a,\"\",c\n1,2,3\n", CatHeader, 0)
	// 列名重复（区分大小写：a 与 A 不冲突，a 与 a 冲突）。
	importErr(t, "a,b,a\n1,2,3\n", CatHeader, 0)
	ds := importOK(t, "a,A\n1,2\n")
	if ds.Columns[0].Name != "a" || ds.Columns[1].Name != "A" {
		t.Fatalf("case-sensitive names: %+v", ds.Columns)
	}
}

func TestColumnCountMismatch(t *testing.T) {
	importErr(t, "a,b\n1\n", CatShape, 1)
	importErr(t, "a,b\n1,2,3\n", CatShape, 1)
	importErr(t, "a,b\n1,2\n3\n", CatShape, 2)
}

func TestLimits(t *testing.T) {
	// 正好 50 列、10000 行可以导入。
	var sb strings.Builder
	head := make([]string, MaxCols)
	row := make([]string, MaxCols)
	for i := range head {
		head[i] = "c" + strings.Repeat("x", 1) + string(rune('A'+i%26)) + strings.Repeat("y", i/26)
		row[i] = "1"
	}
	// 简单生成唯一列名。
	for i := range head {
		head[i] = "col" + itoa(i)
	}
	sb.WriteString(strings.Join(head, ","))
	sb.WriteByte('\n')
	line := strings.Join(row, ",") + "\n"
	for r := 0; r < MaxRows; r++ {
		sb.WriteString(line)
	}
	ds := importOK(t, sb.String())
	if ds.Rows != MaxRows || len(ds.Columns) != MaxCols {
		t.Fatalf("rows=%d cols=%d", ds.Rows, len(ds.Columns))
	}
	// 51 列拒收。
	sb.Reset()
	sb.WriteString(strings.Join(append(head, "col50"), ","))
	sb.WriteByte('\n')
	importErr(t, sb.String(), CatLimit, 0)
	// 10001 行拒收，记录序号为第 10001 条。
	sb.Reset()
	sb.WriteString("a\n")
	for r := 0; r < MaxRows+1; r++ {
		sb.WriteString("1\n")
	}
	importErr(t, sb.String(), CatLimit, MaxRows+1)
}

// itoa 非负整数转十进制字符串（测试辅助，避免引入 strconv 的依赖噪音）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
