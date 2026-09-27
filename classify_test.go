package dataset

import "testing"

func TestClassifyInt(t *testing.T) {
	accept := []string{"0", "-0", "1", "-42", "123456789",
		"9223372036854775807", "-9223372036854775808"} // int64 边界
	for _, s := range accept {
		if isInt, _, _ := classifyValue(s); !isInt {
			t.Errorf("expected %q to be int", s)
		}
	}
	reject := []string{
		"+1",                         // 不能带正号
		"1,000",                      // 不能带千分位
		"007",                        // 多余前导零
		"00",                         // 多余前导零
		"-01",                        // 多余前导零
		"9223372036854775808",        // 超出 int64 上限
		"-9223372036854775809",       // 超出 int64 下限
		"1.0", "abc", "", " 1", "1 ", // 带空格、非数字
	}
	for _, s := range reject {
		if isInt, _, _ := classifyValue(s); isInt {
			t.Errorf("expected %q not to be int", s)
		}
	}
}

func TestClassifyFloat(t *testing.T) {
	accept := []string{
		"1.5", "-0.5", "0.0", "3.14159",
		"1.5e-3", "2.0E+10", "1.0e0", "-2.5E-8",
		"1.7976931348623157e308", // 接近 float64 上限
		"5.0e-324",               // 最小正 denormal
	}
	for _, s := range accept {
		if _, isFloat, _ := classifyValue(s); !isFloat {
			t.Errorf("expected %q to be float", s)
		}
	}
	reject := []string{
		"1.",                 // 小数点右侧没有数字
		".5",                 // 小数点左侧没有数字
		"01.5",               // 整数部分多余前导零
		"1.5e",               // 指数没有数字
		"1e5",                // 没有小数点
		"NaN", "Inf", "-Inf", // NaN、Inf 不算
		"0x1.8p1",      // 十六进制写法不算
		"1.0e400",      // 超出 float64 上限
		"1.0e-400",     // 下溢出 float64 范围
		" 1.5", "1.5 ", // 前后带空格
		"+1.5", // 不能带正号
	}
	for _, s := range reject {
		if _, isFloat, _ := classifyValue(s); isFloat {
			t.Errorf("expected %q not to be float", s)
		}
	}
}

func TestClassifyDate(t *testing.T) {
	accept := []string{
		"1/5/2023", "12/31/1999", "01/05/2023", // 月日可写一位或两位
		"2/29/2020", // 闰日
		"2/29/2000", // 400 的倍数是闰年
		"4/30/2023",
	}
	for _, s := range accept {
		if _, _, isDate := classifyValue(s); !isDate {
			t.Errorf("expected %q to be date", s)
		}
	}
	reject := []string{
		"2/29/2023", // 2023 不是闰年
		"2/29/1900", // 100 的倍数但不是 400 的倍数不是闰年
		"2/30/2020", // 2 月没有 30 日
		"13/1/2023", // 没有 13 月
		"0/15/2023", // 没有 0 月
		"1/0/2023",  // 没有 0 日
		"4/31/2023", // 4 月只有 30 天
		"1/1/99",    // 年必须四位
		"2023/1/1",  // 顺序必须是月/日/年
		"1-1-2023",  // 分隔符必须是 /
		" 1/5/2023", // 前后带空格
	}
	for _, s := range reject {
		if _, _, isDate := classifyValue(s); isDate {
			t.Errorf("expected %q not to be date", s)
		}
	}
}

func TestInferColumnType(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   ColType
	}{
		{"all int", []string{"1", "-2", "0"}, ColInt},
		{"int and float", []string{"1", "1.5", "-2"}, ColFloat},
		{"all float", []string{"1.5", "2.0e3"}, ColFloat},
		{"all date", []string{"1/5/2023", "2/29/2020"}, ColDate},
		{"date and text", []string{"1/5/2023", "hello"}, ColText},
		{"int and text", []string{"1", "abc"}, ColText},
		{"int and date", []string{"1", "1/5/2023"}, ColText},
		{"all missing", nil, ColText},
		{"text keeps spaces", []string{" 1", "2"}, ColText},
		{"zip codes", []string{"02139", "02140"}, ColText}, // 邮编不当数字
		{"empty string is text", []string{""}, ColText},
	}
	for _, c := range cases {
		if got := inferColumnType(c.values); got != c.want {
			t.Errorf("%s: expected %v, got %v", c.name, c.want, got)
		}
	}
}
