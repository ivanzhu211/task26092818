package dataset

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// nastyTexts 文本单元格池：逗号、引号、回车、换行、首尾空格、空串等。
var nastyTexts = []string{
	"", " ", "  ", " lead", "trail ", " both ",
	"a,b", ",start", "end,", "a,,b",
	`say "hi"`, `"quoted"`, `a"b"c`,
	"line1\nline2", "line1\r\nline2", "l1\rl2", "\n", "\r\n",
	"tab\there", "中文内容", "emoji🎉", "\uFEFFbom-inside",
	"007", "+1", "1,000", "2/29/2023", "NaN", "Inf", "0x1p3",
	" 1", "1 ", "1.5 ", " 1/1/2020",
	"mixed, \"all\"\r\nthings ",
}

var intPool = []string{
	"0", "-0", "1", "-1", "42", "-42", "1000000",
	"9223372036854775807", "-9223372036854775808",
}

var floatPool = []string{
	"0.0", "-0.0", "1.5", "-1.5", "3.0", "-2.0", // 整数写法的小数
	"1.5e-3", "2.0E+10", "0.1", "123456.789",
	"1.7976931348623157e308", "-1.7976931348623157e308", // 极大小数
	"4.9e-324", "2.5e-323", // 极小小数
}

var datePool = []string{
	"1/1/2000", "12/31/1999", "2/29/2024", "02/29/2024", // 闰日
	"11/9/2023", "01/01/1900",
}

// genDataset 生成一个随机 CSV 数据集及其期望的单元格矩阵。
func genDataset(r *rand.Rand) (csv string, header []string, cells [][]*string) {
	cols := 1 + r.Intn(8) // 1~8 列，含只有一列的情况
	header = make([]string, cols)
	used := map[string]bool{}
	for c := 0; c < cols; {
		name := "col" + strconv.Itoa(c) + "_" + strconv.Itoa(r.Intn(1000))
		if !used[name] && name != "" {
			used[name] = true
			header[c] = name
			c++
		}
	}
	rows := r.Intn(51) // 0~50 行
	cells = make([][]*string, rows)
	// 每列选一种形态，保证能产出各类型列。
	for c := 0; c < cols; c++ {
		kind := r.Intn(5) // 0 int, 1 float, 2 date, 3 text, 4 mixed
		for row := 0; row < rows; row++ {
			if cells[row] == nil {
				cells[row] = make([]*string, cols)
			}
			if r.Intn(10) == 0 { // 10% 缺失
				cells[row][c] = nil
				continue
			}
			var v string
			switch kind {
			case 0:
				v = intPool[r.Intn(len(intPool))]
			case 1:
				v = floatPool[r.Intn(len(floatPool))]
			case 2:
				v = datePool[r.Intn(len(datePool))]
			case 3:
				v = nastyTexts[r.Intn(len(nastyTexts))]
			default:
				pools := [][]string{intPool, floatPool, datePool, nastyTexts}
				p := pools[r.Intn(len(pools))]
				v = p[r.Intn(len(p))]
			}
			vv := v
			cells[row][c] = &vv
		}
	}
	// 序列化为 CSV（与导出同规则）。
	var sb strings.Builder
	sb.WriteString(joinRow(header))
	for _, row := range cells {
		sb.WriteByte('\n')
		sb.WriteString(joinCells(row))
	}
	sb.WriteByte('\n')
	return sb.String(), header, cells
}

func joinRow(names []string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = escapeField(n)
	}
	return strings.Join(parts, ",")
}

func joinCells(row []*string) string {
	parts := make([]string, len(row))
	for i, c := range row {
		if c != nil {
			parts[i] = escapeField(*c)
		}
	}
	return strings.Join(parts, ",")
}

func escapeField(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, ",\"\r\n") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// assertSameDataset 断言两次导入得到完全相同的数据集。
func assertSameDataset(t *testing.T, st *Store, id1, id2 int64) {
	t.Helper()
	s1, err := st.Summary(id1)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := st.Summary(id2)
	if err != nil {
		t.Fatal(err)
	}
	if s1.Rows != s2.Rows || len(s1.Columns) != len(s2.Columns) {
		t.Fatalf("shape differs: %d/%d rows, %d/%d cols",
			s1.Rows, s2.Rows, len(s1.Columns), len(s2.Columns))
	}
	for i := range s1.Columns {
		a, b := s1.Columns[i], s2.Columns[i]
		if a.Name != b.Name || a.Type != b.Type || a.Missing != b.Missing {
			t.Fatalf("column %d differs: %+v vs %+v", i, a, b)
		}
	}
	for r := 0; r < s1.Rows; r++ {
		r1, err := st.Row(id1, r)
		if err != nil {
			t.Fatal(err)
		}
		r2, err := st.Row(id2, r)
		if err != nil {
			t.Fatal(err)
		}
		for c := range r1 {
			a, b := r1[c], r2[c]
			if a.Null != b.Null {
				t.Fatalf("cell (%d,%d): null differs %v vs %v", r, c, a, b)
			}
			if a.Null {
				continue
			}
			if s1.Columns[c].Type == TypeFloat {
				// 小数按 64 位浮点值相等。
				fa, _ := strconv.ParseFloat(a.Value, 64)
				fb, _ := strconv.ParseFloat(b.Value, 64)
				if fa != fb {
					t.Fatalf("cell (%d,%d): float %v != %v", r, c, a.Value, b.Value)
				}
			} else if a.Value != b.Value {
				t.Fatalf("cell (%d,%d): %q != %q", r, c, a.Value, b.Value)
			}
		}
	}
}

// roundTrip 导入→导出→再导入，断言完全一致，且两次导出字节相同。
func roundTrip(t *testing.T, st *Store, csv string) {
	t.Helper()
	d1, err := st.Import("a", []byte(csv))
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	exp1, err := st.Export(d1.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	d2, err := st.Import("b", exp1)
	if err != nil {
		t.Fatalf("re-import of exported CSV failed: %v\ncsv:\n%s", err, exp1)
	}
	assertSameDataset(t, st, d1.ID, d2.ID)
	exp2, err := st.Export(d2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(exp1) != string(exp2) {
		t.Fatalf("exports not byte-identical")
	}
}

func TestRoundTripRandom(t *testing.T) {
	st, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	r := rand.New(rand.NewSource(20260928))
	for i := 0; i < 200; i++ {
		csv, _, _ := genDataset(r)
		roundTrip(t, st, csv)
	}
}

func TestRoundTripSingleColumn(t *testing.T) {
	st, _ := Open(":memory:")
	defer st.Close()
	roundTrip(t, st, "only\n1\n\n\"\"\n\"a,b\"\n\"x\"\"y\"\n")
}

func TestRoundTripHeaderOnly(t *testing.T) {
	st, _ := Open(":memory:")
	defer st.Close()
	roundTrip(t, st, "a,b,c\n")
	roundTrip(t, st, "a,b,c") // 无末尾换行
}

func TestRoundTripNastyTexts(t *testing.T) {
	st, _ := Open(":memory:")
	defer st.Close()
	var sb strings.Builder
	sb.WriteString("txt\n")
	for _, s := range nastyTexts {
		sb.WriteString(escapeField(s))
		sb.WriteByte('\n')
	}
	roundTrip(t, st, sb.String())
}
