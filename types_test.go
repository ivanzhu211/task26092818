package dataset

import "testing"

func TestIsInt(t *testing.T) {
	accept := []string{
		"0", "-0", "1", "-1", "42", "-42",
		"9223372036854775807",  // int64 最大值
		"-9223372036854775808", // int64 最小值
	}
	reject := []string{
		"", "+1", "1,000", "007", "01", "-01", "00",
		"9223372036854775808",  // 超出 int64
		"-9223372036854775809", // 超出 int64
		"1.0", "1e3", " 1", "1 ", "--1", "1-", "abc",
	}
	for _, s := range accept {
		if !isInt(s) {
			t.Errorf("isInt(%q) should be true", s)
		}
	}
	for _, s := range reject {
		if isInt(s) {
			t.Errorf("isInt(%q) should be false", s)
		}
	}
}

func TestIsFloat(t *testing.T) {
	accept := []string{
		"0.0", "1.5", "-1.5", "0.5", "-0.5", "10.25",
		"1.5e-3", "2.0E+10", "1.0e0", "-3.25E-2",
		"1.7976931348623157e308", // 接近 float64 最大值
		"4.9e-324", // 最小正 denormal 量级
	}
	reject := []string{
		"", "1", "1.", ".5", "-.5", "01.5", "00.5", "+1.5",
		"1.5e", "1.5e+", "1.5e1.5", "1.5E",
		"NaN", "nan", "Inf", "+Inf", "-Inf", "Infinity",
		"0x1.8p1", "0x1p-2", // 十六进制写法
		"1.5e999", "-1.5e999", // 超出 float64 范围
		" 1.5", "1.5 ", "1,5", "1..5", "1.5.6",
	}
	for _, s := range accept {
		if !isFloat(s) {
			t.Errorf("isFloat(%q) should be true", s)
		}
	}
	for _, s := range reject {
		if isFloat(s) {
			t.Errorf("isFloat(%q) should be false", s)
		}
	}
}

func TestIsDate(t *testing.T) {
	accept := []string{
		"1/1/2000", "01/01/2000", "12/31/1999", "2/29/2024", // 闰日
		"02/29/2024", "11/9/2023", "1/01/2023",
	}
	reject := []string{
		"", "2/29/2023", // 2023 不是闰年
		"2/30/2024", "4/31/2024", "13/1/2020", "0/1/2020", "1/0/2020",
		"1/32/2020", "2/29/1900", // 1900 不是闰年
		"2/3/21", "2/3/20233", "2023/1/2", "1-1-2020", "1/1/2020 ",
		" 1/1/2020", "1//2020", "/1/2020", "1/1/",
	}
	for _, s := range accept {
		if !isDate(s) {
			t.Errorf("isDate(%q) should be true", s)
		}
	}
	for _, s := range reject {
		if isDate(s) {
			t.Errorf("isDate(%q) should be false", s)
		}
	}
}

// 通过导入整列来验证列类型推断。
func colType(t *testing.T, vals ...string) ColumnType {
	t.Helper()
	csv := "c\n"
	for _, v := range vals {
		csv += v + "\n"
	}
	ds := importOK(t, csv)
	return ds.Columns[0].Type
}

func TestColumnTypeInference(t *testing.T) {
	cases := []struct {
		vals []string
		want ColumnType
	}{
		{[]string{"1", "2", "-3"}, TypeInt},
		{[]string{"1", "" /*缺失行由空行表示，见下*/}, TypeInt}, // 占位，实际见 TestMissingCells
		{[]string{"1", "2.5"}, TypeFloat},
		{[]string{"1.5", "2.5", "-3"}, TypeFloat},
		{[]string{"1/1/2020", "2/29/2024"}, TypeDate},
		{[]string{"1", "1/1/2020"}, TypeText},   // 整数混日期 → 文本
		{[]string{"1.5", "1/1/2020"}, TypeText}, // 小数混日期 → 文本
		{[]string{"abc", "1"}, TypeText},
		{[]string{"abc", "def"}, TypeText},
		{[]string{"1", " 2"}, TypeText}, // 带空格的数字按文本
		{[]string{"\"\"", "1"}, TypeText}, // 空串是文本值
	}
	for _, c := range cases {
		got := colType(t, c.vals...)
		if got != c.want {
			t.Errorf("vals %v: want %v, got %v", c.vals, c.want, got)
		}
	}
}

func TestMissingCellsAndInference(t *testing.T) {
	// 缺失值不参与类型判断。
	ds := importOK(t, "a\n1\n\n2\n")
	if ds.Columns[0].Type != TypeInt || ds.Columns[0].Missing != 1 {
		t.Fatalf("%+v", ds.Columns[0])
	}
	// 整列缺失 → 文本列。
	ds = importOK(t, "a,b\n,\n,\n")
	if ds.Columns[0].Type != TypeText || ds.Columns[0].Missing != 2 {
		t.Fatalf("%+v", ds.Columns[0])
	}
	// 数字前后带空格 → 文本列，且空格原样保留。
	st, _ := Open(":memory:")
	defer st.Close()
	d2, err := st.Import("t", []byte("a\n 1 \n"))
	if err != nil {
		t.Fatal(err)
	}
	if d2.Columns[0].Type != TypeText {
		t.Fatalf("spaced number should be text: %+v", d2.Columns[0])
	}
	c, _ := st.Cell(d2.ID, 0, 0)
	if c.Value != " 1 " {
		t.Fatalf("spaces must be preserved: %q", c.Value)
	}
}
