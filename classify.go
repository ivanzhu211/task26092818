package dataset

import "strconv"

// classifyValue 判断单个非缺失值分别是否属于整数、小数、日期。
// 前后带空格的值在这里全部不匹配，自然落入文本。
func classifyValue(s string) (intOK, floatOK, dateOK bool) {
	return isInt64(s), isFloat64(s), isDate(s)
}

// isInt64 报告 s 是否是规范的 64 位有符号整数写法：
// 可带负号，不能带正号或千分位，不能有多余前导零（「0」本身可以）。
func isInt64(s string) bool {
	digits := s
	if len(digits) > 0 && digits[0] == '-' {
		digits = digits[1:]
	}
	if !isUintDigits(digits) {
		return false
	}
	if _, err := strconv.ParseInt(s, 10, 64); err != nil {
		return false // 超出 64 位有符号范围
	}
	return true
}

// isUintDigits 报告 s 是否是不带符号、无多余前导零的数字串。
func isUintDigits(s string) bool {
	if s == "0" {
		return true
	}
	if len(s) == 0 || s[0] < '1' || s[0] > '9' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isFloat64 报告 s 是否是规范的小数写法：可带负号，必须有小数点且两侧
// 都至少一位数字，整数部分无多余前导零，可跟指数；须在 64 位浮点范围内。
func isFloat64(s string) bool {
	body := s
	if len(body) > 0 && body[0] == '-' {
		body = body[1:]
	}
	mantissa := body
	exp := ""
	hasExp := false
	for i := 0; i < len(body); i++ {
		if body[i] == 'e' || body[i] == 'E' {
			mantissa = body[:i]
			exp = body[i+1:]
			hasExp = true
			break
		}
	}
	// 尾数：整数部分.小数部分，两侧都至少一位数字
	dot := -1
	for i := 0; i < len(mantissa); i++ {
		if mantissa[i] == '.' {
			if dot >= 0 {
				return false
			}
			dot = i
		}
	}
	if dot <= 0 || dot == len(mantissa)-1 {
		return false
	}
	if !isUintDigits(mantissa[:dot]) {
		return false
	}
	if !allDigits(mantissa[dot+1:]) {
		return false
	}
	if hasExp {
		if len(exp) > 0 && (exp[0] == '+' || exp[0] == '-') {
			exp = exp[1:]
		}
		if !allDigits(exp) {
			return false
		}
	}
	// 超出 64 位浮点范围（上溢为 Inf、下溢为 0）的不算
	if f, err := strconv.ParseFloat(s, 64); err != nil {
		return false
	} else if f == 0 && mantissaHasNonZero(mantissa) {
		return false
	}
	return true
}

func mantissaHasNonZero(mantissa string) bool {
	for i := 0; i < len(mantissa); i++ {
		if mantissa[i] >= '1' && mantissa[i] <= '9' {
			return true
		}
	}
	return false
}

func allDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isDate 报告 s 是否是「月/日/四位年」的真实日期，月和日可写一位或两位。
func isDate(s string) bool {
	var parts [3]string
	idx := 0
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '/' {
			if idx >= 3 {
				return false
			}
			parts[idx] = s[start:i]
			idx++
			start = i + 1
		}
	}
	if idx != 3 {
		return false
	}
	month, day, year := parts[0], parts[1], parts[2]
	if len(month) < 1 || len(month) > 2 || !allDigits(month) {
		return false
	}
	if len(day) < 1 || len(day) > 2 || !allDigits(day) {
		return false
	}
	if len(year) != 4 || !allDigits(year) {
		return false
	}
	m, _ := strconv.Atoi(month)
	d, _ := strconv.Atoi(day)
	y, _ := strconv.Atoi(year)
	if m < 1 || m > 12 {
		return false
	}
	return d >= 1 && d <= daysInMonth(m, y)
}

func daysInMonth(month, year int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	default: // 2 月
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	}
}

// inferColumnType 根据一列所有非缺失值推断列类型：全是整数为整数列，
// 全是整数或小数为小数列，全是日期为日期列，其余（包括整列缺失）为文本列。
func inferColumnType(values []string) ColType {
	if len(values) == 0 {
		return ColText
	}
	allInt, allNumeric, allDate := true, true, true
	for _, v := range values {
		isInt, isFloat, isDate := classifyValue(v)
		if !isInt {
			allInt = false
		}
		if !isInt && !isFloat {
			allNumeric = false
		}
		if !isDate {
			allDate = false
		}
	}
	switch {
	case allInt:
		return ColInt
	case allNumeric:
		return ColFloat
	case allDate:
		return ColDate
	default:
		return ColText
	}
}
