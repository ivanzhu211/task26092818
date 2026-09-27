package dataset

// inferColumnType 根据一列所有非缺失单元格的文本推断列类型。
// cells 为该列全部单元格（nil 表示缺失）。
// 规则：所有非缺失值都是整数 → 整数列；都是整数或小数 → 小数列；
// 都是日期 → 日期列；其余情况（包括整列缺失）为文本列。
func inferColumnType(cells []*string) ColumnType {
	allInt, allNum, allDate, seen := true, true, true, false
	for _, c := range cells {
		if c == nil {
			continue
		}
		seen = true
		s := *c
		if !isInt(s) {
			allInt = false
		}
		if !isInt(s) && !isFloat(s) {
			allNum = false
		}
		if !isDate(s) {
			allDate = false
		}
	}
	if !seen {
		return TypeText // 整列缺失按文本
	}
	switch {
	case allInt:
		return TypeInt
	case allNum:
		return TypeFloat
	case allDate:
		return TypeDate
	default:
		return TypeText
	}
}
