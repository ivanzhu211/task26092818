package dataset

import (
	"strings"
	"testing"
)

func parseOK(t *testing.T, input string) []record {
	t.Helper()
	recs, ierr := parseCSV([]byte(input))
	if ierr != nil {
		t.Fatalf("expected success, got %v", ierr)
	}
	return recs
}

func parseErr(t *testing.T, input string, cat ErrCategory, recNo int) {
	t.Helper()
	_, ierr := parseCSV([]byte(input))
	if ierr == nil {
		t.Fatalf("expected error %s, got success", cat)
	}
	if ierr.Category != cat || ierr.Record != recNo {
		t.Fatalf("expected %s at record %d, got %s at record %d", cat, recNo, ierr.Category, ierr.Record)
	}
}

func fieldVal(t *testing.T, recs []record, rec, col int, want *string) {
	t.Helper()
	got := recs[rec][col]
	if want == nil {
		if got != nil {
			t.Fatalf("record %d col %d: expected missing, got %q", rec, col, *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Fatalf("record %d col %d: expected %q, got %v", rec, col, *want, got)
	}
}

func strptr(s string) *string { return &s }

func TestLineEndings(t *testing.T) {
	// LF、CRLF、同一文件混用
	recs := parseOK(t, "a,b\n1,2\r\n3,4\n")
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	fieldVal(t, recs, 2, 1, strptr("4"))
}

func TestBOMIgnored(t *testing.T) {
	recs := parseOK(t, "\xEF\xBB\xBFa,b\n1,2\n")
	fieldVal(t, recs, 0, 0, strptr("a"))
}

func TestTrailingNewlineAndEmptyLines(t *testing.T) {
	// 文件末尾的一个换行不产生新行
	recs := parseOK(t, "a\n1\n")
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	// 其余空行都算一行数据（单列时是一个缺失单元格）
	recs = parseOK(t, "a\n1\n\n2\n")
	if len(recs) != 4 {
		t.Fatalf("expected 4 records, got %d", len(recs))
	}
	fieldVal(t, recs, 2, 0, nil)
	// 末尾两个换行：倒数第二个换行产生一个空行
	recs = parseOK(t, "a\n1\n\n")
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	fieldVal(t, recs, 2, 0, nil)
	// 没有末尾换行也可以
	recs = parseOK(t, "a\n1")
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
}

func TestQuotedFields(t *testing.T) {
	// 含逗号、引号、换行的单元格
	recs := parseOK(t, "a,b,c\n\"x,y\",\"he said \"\"hi\"\"\",\"line1\nline2\"\n")
	fieldVal(t, recs, 1, 0, strptr("x,y"))
	fieldVal(t, recs, 1, 1, strptr(`he said "hi"`))
	fieldVal(t, recs, 1, 2, strptr("line1\nline2"))
	// 引号内的回车和 CRLF 原样保留
	recs = parseOK(t, "a\n\"x\ry\r\nz\"\n")
	fieldVal(t, recs, 1, 0, strptr("x\ry\r\nz"))
	// 引号字段后正好到文件结束
	recs = parseOK(t, "a\n\"x\"")
	fieldVal(t, recs, 1, 0, strptr("x"))
}

func TestBadQuotes(t *testing.T) {
	// 未加引号的单元格里出现引号
	parseErr(t, "a\nx\"y\n", ErrBadQuote, 1)
	// 闭合引号后紧跟其他字符
	parseErr(t, "a\n\"x\"y\n", ErrBadQuote, 1)
	parseErr(t, "a,b\n\"x\" ,2\n", ErrBadQuote, 1)
	// 引号到文件结束都没闭合
	parseErr(t, "a\n\"x\n", ErrBadQuote, 1)
	parseErr(t, "a,b\n1,\"unclosed", ErrBadQuote, 1)
}

func TestMissingVsEmptyString(t *testing.T) {
	recs := parseOK(t, "a,b\n,\"\"\n")
	fieldVal(t, recs, 1, 0, nil)        // 空单元格是缺失值
	fieldVal(t, recs, 1, 1, strptr("")) // 「""」是空字符串
}

func TestHeaderRules(t *testing.T) {
	// 列名不能为空
	parseErr(t, "a,,c\n1,2,3\n", ErrEmptyHeader, 0)
	parseErr(t, "a,\"\",c\n1,2,3\n", ErrEmptyHeader, 0)
	// 列名不能重复（区分大小写）
	parseErr(t, "a,b,a\n1,2,3\n", ErrDupHeader, 0)
	parseOK(t, "a,A\n1,2\n") // 大小写不同不算重复
}

func TestColumnCount(t *testing.T) {
	parseErr(t, "a,b\n1\n", ErrColumnCount, 1)
	parseErr(t, "a,b\n1,2,3\n", ErrColumnCount, 1)
	// 出错位置是记录序号
	parseErr(t, "a,b\n1,2\n3\n", ErrColumnCount, 2)
}

func TestSizeLimits(t *testing.T) {
	// 正好 50 列可以
	var sb strings.Builder
	for i := 0; i < maxCols; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('a' + byte(i%26))
		sb.WriteString(strings.Repeat("x", i/26))
	}
	sb.WriteByte('\n')
	header := sb.String()
	row := strings.Repeat("1,", maxCols-1) + "1\n"
	parseOK(t, header+row)

	// 51 列拒收
	sb.WriteString("extra\n")
	parseErr(t, "a,"+strings.Repeat("b,", maxCols-1)+"c\n"+row, ErrTooManyCols, 0)

	// 正好 10000 行数据可以
	var big strings.Builder
	big.WriteString("a\n")
	for i := 0; i < maxRows; i++ {
		big.WriteString("1\n")
	}
	parseOK(t, big.String())

	// 10001 行拒收，出错位置是第 10001 条记录
	big.WriteString("1\n")
	parseErr(t, big.String(), ErrTooManyRows, maxRows+1)
}

func TestInvalidUTF8(t *testing.T) {
	// 不是合法 UTF-8 整份拒收，记录序号指向出错记录
	_, ierr := parseCSV([]byte("a,b\n1,\xff\n"))
	if ierr == nil || ierr.Category != ErrNotUTF8 || ierr.Record != 1 {
		t.Fatalf("expected not_utf8 at record 1, got %v", ierr)
	}
	// 表头里的非法 UTF-8
	_, ierr = parseCSV([]byte("\xfe\n"))
	if ierr == nil || ierr.Category != ErrNotUTF8 || ierr.Record != 0 {
		t.Fatalf("expected not_utf8 at record 0, got %v", ierr)
	}
}

func TestLoneCarriageReturn(t *testing.T) {
	// 引号外孤立的 CR 不是合法行尾
	parseErr(t, "a\r1\n", ErrBadLineEnd, 0)
}

func TestEmptyFile(t *testing.T) {
	parseErr(t, "", ErrEmptyDataset, 0)
}
