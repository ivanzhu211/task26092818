package dataset

import "unicode/utf8"

// field 为 nil 表示缺失值，非 nil 表示单元格内容（可能是空字符串）。
type record []*string

const (
	maxRows = 10000
	maxCols = 50
)

// parseCSV 解析整份 CSV，返回所有记录（含表头，表头为第 0 条）。
// 解析出的记录已经过全部格式校验，可直接用于建数据集。
func parseCSV(data []byte) ([]record, *ImportError) {
	// 文件开头的 UTF-8 BOM 忽略
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	var recs []record
	pos := 0
	for pos < len(data) {
		rec, next, ierr := parseRecord(data, pos, len(recs))
		if ierr != nil {
			return nil, ierr
		}
		recs = append(recs, rec)
		pos = next
	}
	if len(recs) == 0 {
		return nil, &ImportError{Category: ErrEmptyDataset, Record: 0, Detail: "file contains no records"}
	}

	// UTF-8 校验：结构字符都是 ASCII，非法字节只会落在字段内容里
	for i, rec := range recs {
		for _, f := range rec {
			if f != nil && !utf8.ValidString(*f) {
				return nil, &ImportError{Category: ErrNotUTF8, Record: i, Detail: "field is not valid UTF-8"}
			}
		}
	}

	header := recs[0]
	if len(header) > maxCols {
		return nil, &ImportError{Category: ErrTooManyCols, Record: 0, Detail: "more than 50 columns"}
	}
	seen := make(map[string]bool, len(header))
	for _, f := range header {
		if f == nil || *f == "" {
			return nil, &ImportError{Category: ErrEmptyHeader, Record: 0, Detail: "column name is empty"}
		}
		if seen[*f] {
			return nil, &ImportError{Category: ErrDupHeader, Record: 0, Detail: "duplicate column name: " + *f}
		}
		seen[*f] = true
	}

	for i := 1; i < len(recs); i++ {
		if i > maxRows {
			return nil, &ImportError{Category: ErrTooManyRows, Record: i, Detail: "more than 10000 data rows"}
		}
		if len(recs[i]) != len(header) {
			return nil, &ImportError{Category: ErrColumnCount, Record: i, Detail: "record has different column count than header"}
		}
	}
	return recs, nil
}

// 字段结束方式
const (
	endComma = iota
	endNewline
	endEOF
)

// parseRecord 从 pos 开始解析一条记录，返回字段列表和下一条记录的起始位置。
func parseRecord(data []byte, pos, recNo int) (record, int, *ImportError) {
	var fields record
	for {
		f, next, end, ierr := parseField(data, pos, recNo)
		if ierr != nil {
			return nil, 0, ierr
		}
		fields = append(fields, f)
		pos = next
		if end == endComma {
			continue
		}
		return fields, pos, nil
	}
}

// parseField 从 pos 开始解析一个字段，返回字段值、字段结束后的位置（跳过
// 分隔符或行尾）以及结束方式。
func parseField(data []byte, pos, recNo int) (*string, int, int, *ImportError) {
	if pos < len(data) && data[pos] == '"' {
		return parseQuotedField(data, pos, recNo)
	}
	start := pos
	for pos < len(data) {
		switch c := data[pos]; c {
		case ',':
			return unquoted(data[start:pos]), pos + 1, endComma, nil
		case '\n':
			return unquoted(data[start:pos]), pos + 1, endNewline, nil
		case '\r':
			if pos+1 < len(data) && data[pos+1] == '\n' {
				return unquoted(data[start:pos]), pos + 2, endNewline, nil
			}
			return nil, 0, 0, &ImportError{Category: ErrBadLineEnd, Record: recNo, Detail: "lone carriage return outside quotes"}
		case '"':
			return nil, 0, 0, &ImportError{Category: ErrBadQuote, Record: recNo, Detail: "quote in unquoted field"}
		}
		pos++
	}
	return unquoted(data[start:pos]), pos, endEOF, nil
}

// unquoted 把未加引号的字段内容转成字段值：空内容为缺失值。
func unquoted(b []byte) *string {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	return &s
}

// parseQuotedField 解析加引号的字段，pos 指向开头的双引号。
func parseQuotedField(data []byte, pos, recNo int) (*string, int, int, *ImportError) {
	pos++ // 跳过开头引号
	var buf []byte
	for {
		if pos >= len(data) {
			return nil, 0, 0, &ImportError{Category: ErrBadQuote, Record: recNo, Detail: "unterminated quoted field"}
		}
		c := data[pos]
		if c != '"' {
			// 引号内的回车、换行等原样属于单元格内容
			buf = append(buf, c)
			pos++
			continue
		}
		// 遇到引号：两个连续引号是转义，否则是闭合引号
		if pos+1 < len(data) && data[pos+1] == '"' {
			buf = append(buf, '"')
			pos += 2
			continue
		}
		pos++ // 跳过闭合引号
		if pos >= len(data) {
			s := string(buf)
			return &s, pos, endEOF, nil
		}
		switch data[pos] {
		case ',':
			s := string(buf)
			return &s, pos + 1, endComma, nil
		case '\n':
			s := string(buf)
			return &s, pos + 1, endNewline, nil
		case '\r':
			if pos+1 < len(data) && data[pos+1] == '\n' {
				s := string(buf)
				return &s, pos + 2, endNewline, nil
			}
			return nil, 0, 0, &ImportError{Category: ErrBadQuote, Record: recNo, Detail: "unexpected character after closing quote"}
		default:
			return nil, 0, 0, &ImportError{Category: ErrBadQuote, Record: recNo, Detail: "unexpected character after closing quote"}
		}
	}
}
