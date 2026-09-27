package dataset

import "unicode/utf8"

// parseCSV 严格解析整份 CSV 字节流。
// 返回记录列表（records[0] 为表头）；每条记录是单元格切片，
// nil 表示缺失值（空单元格），非 nil 为单元格内容
// （带引号的空串 "" 解析为指向空字符串，与缺失值区分）。
// 任何格式违规都整份拒收，返回 *ImportError。
//
// 词法规则：
//   - 分隔符为逗号；行尾可 LF 或 CRLF 混用；文件开头 UTF-8 BOM 忽略。
//   - 文件末尾的一个换行不产生新行，其余空行都算一条记录。
//   - 含逗号、引号、换行的单元格用双引号包住，内部引号写成两个；
//     引号内的回车换行原样属于单元格内容。
//   - 未加引号的单元格出现引号、闭合引号后紧跟其他字符、
//     引号到文件结束未闭合，均整份拒收。
//   - 全文必须是合法 UTF-8。
func parseCSV(data []byte) ([][]*string, *ImportError) {
	i := 0
	// 忽略文件开头的 UTF-8 BOM。
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		i = 3
	}

	var records [][]*string
	record := 0 // 当前记录序号：表头为 0，其后第一条数据记录为 1

	for i < len(data) {
		var cells []*string
		for {
			cell, next, err := parseCell(data, i, record)
			if err != nil {
				return nil, err
			}
			cells = append(cells, cell)
			i = next
			if i < len(data) && data[i] == ',' {
				i++
				continue
			}
			break
		}
		// 行尾：LF、CRLF 或文件结束；单独的 CR 不合法。
		if i < len(data) {
			switch {
			case data[i] == '\n':
				i++
			case data[i] == '\r':
				if i+1 < len(data) && data[i+1] == '\n' {
					i += 2
				} else {
					return nil, newErr(CatSyntax, record, "lone carriage return")
				}
			}
		}
		records = append(records, cells)
		record++
	}
	return records, nil
}

// parseCell 从 data[i] 开始解析一个单元格，
// 返回单元格值（nil 表示缺失）与单元格结束后的位置
// （停在 ','、'\n'、'\r' 或文件结束处）。
func parseCell(data []byte, i int, record int) (*string, int, *ImportError) {
	if i < len(data) && data[i] == '"' {
		return parseQuotedCell(data, i, record)
	}
	// 未加引号的单元格：扫描到分隔符或行尾。
	start := i
	for i < len(data) {
		c := data[i]
		if c == ',' || c == '\n' || c == '\r' {
			break
		}
		if c == '"' {
			return nil, 0, newErr(CatSyntax, record, "bare quote in unquoted field")
		}
		var size int
		var ok bool
		if size, ok = decodeRune(data, i); !ok {
			return nil, 0, newErr(CatEncoding, record, "invalid UTF-8")
		}
		i += size
	}
	if i == start {
		return nil, i, nil // 空单元格：缺失值
	}
	s := string(data[start:i])
	return &s, i, nil
}

// parseQuotedCell 解析带引号的单元格，data[i] 必须是开引号。
func parseQuotedCell(data []byte, i int, record int) (*string, int, *ImportError) {
	i++ // 跳过开引号
	var buf []byte
	for {
		if i >= len(data) {
			return nil, 0, newErr(CatSyntax, record, "unterminated quoted field")
		}
		c := data[i]
		if c == '"' {
			if i+1 < len(data) && data[i+1] == '"' {
				buf = append(buf, '"')
				i += 2
				continue
			}
			i++ // 闭合引号
			break
		}
		size, ok := decodeRune(data, i)
		if !ok {
			return nil, 0, newErr(CatEncoding, record, "invalid UTF-8")
		}
		buf = append(buf, data[i:i+size]...) // 引号内的回车换行原样保留
		i += size
	}
	// 闭合引号后只能是逗号、行尾或文件结束。
	if i < len(data) {
		switch data[i] {
		case ',', '\n':
		case '\r':
			if i+1 >= len(data) || data[i+1] != '\n' {
				return nil, 0, newErr(CatSyntax, record, "unexpected character after closing quote")
			}
		default:
			return nil, 0, newErr(CatSyntax, record, "unexpected character after closing quote")
		}
	}
	s := string(buf)
	return &s, i, nil
}

// decodeRune 解码 data[i] 处的一个 UTF-8 字符，返回字节数与合法性。
func decodeRune(data []byte, i int) (int, bool) {
	if data[i] < utf8.RuneSelf {
		return 1, true
	}
	_, size := utf8.DecodeRune(data[i:])
	if size == 1 { // 非法编码时 DecodeRune 返回 (RuneError, 1)
		return 0, false
	}
	return size, true
}
