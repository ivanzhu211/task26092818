package dataset

import (
	"strconv"
	"strings"
	"time"
)

// isInt 判断文本是否为合法整数写法：
// 可带负号，不带正号/千分位，无多余前导零（"0" 本身可以），
// 且在 int64 范围内。
func isInt(s string) bool {
	digits := strings.TrimPrefix(s, "-")
	if digits == "" || !isDigits(digits) {
		return false
	}
	if len(digits) > 1 && digits[0] == '0' {
		return false // 多余前导零
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

// isFloat 判断文本是否为合法小数写法：
// 可带负号，必须有小数点且两侧至少一位数字，整数部分无多余前导零，
// 可跟指数（如 1.5e-3、2.0E+10），且在 float64 范围内。
// NaN、Inf、十六进制写法不满足语法，自然排除。
func isFloat(s string) bool {
	mantissa := s
	// 可选指数部分。
	if k := strings.IndexAny(s, "eE"); k >= 0 {
		exp := s[k+1:]
		exp = strings.TrimPrefix(exp, "+")
		exp = strings.TrimPrefix(exp, "-")
		if exp == "" || !isDigits(exp) {
			return false
		}
		mantissa = s[:k]
	}
	intFrac := strings.TrimPrefix(mantissa, "-")
	dot := strings.IndexByte(intFrac, '.')
	if dot < 0 {
		return false // 必须有小数点
	}
	intPart, fracPart := intFrac[:dot], intFrac[dot+1:]
	if intPart == "" || fracPart == "" || !isDigits(intPart) || !isDigits(fracPart) {
		return false // 小数点两侧都至少一位数字
	}
	if len(intPart) > 1 && intPart[0] == '0' {
		return false // 整数部分多余前导零
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return false // 超出 float64 范围（上溢/下溢）
	}
	return f == f // 防御 NaN（语法上已排除）
}

// isDate 判断文本是否为合法日期写法：月/日/四位年，
// 月和日可一位或两位，必须是真实存在的日期。
func isDate(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return false
	}
	mo, d, y := parts[0], parts[1], parts[2]
	if !isDigitsN(mo, 1, 2) || !isDigitsN(d, 1, 2) || !isDigitsN(y, 4, 4) {
		return false
	}
	m, _ := strconv.Atoi(mo)
	day, _ := strconv.Atoi(d)
	year, _ := strconv.Atoi(y)
	if m < 1 || m > 12 {
		return false
	}
	// time.Date 会把越界日期进位规范化，回比各分量即可验证真实存在。
	t := time.Date(year, time.Month(m), day, 0, 0, 0, 0, time.UTC)
	return t.Year() == year && int(t.Month()) == m && t.Day() == day
}

// isDigits 判断 s 是否全为 ASCII 数字且非空。
func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// isDigitsN 判断 s 是否为长度在 [min, max] 内的纯数字串。
func isDigitsN(s string, min, max int) bool {
	return len(s) >= min && len(s) <= max && isDigits(s)
}
