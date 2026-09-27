package dataset

import (
	"bufio"
	"bytes"
	"database/sql"
	"strings"
)

// Export 将数据集导出为 CSV 字节流。
// 单元格文本按原样写出，仅在含逗号、引号、回车、换行或为空串时加引号，
// 内部引号写成两个；缺失值写为空单元格。
// 因此导出的文件再导入可得到完全相同的数据集。
func (s *Store) Export(id int64) ([]byte, error) {
	ds, err := s.Summary(id)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := bufio.NewWriterSize(&buf, 1<<16)

	// 表头。
	for i, c := range ds.Columns {
		if i > 0 {
			w.WriteByte(',')
		}
		writeField(w, c.Name)
	}
	w.WriteByte('\n')

	rows, err := s.db.Query(
		`SELECT row, col, value FROM cells WHERE dataset_id = ? ORDER BY row, col`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	curRow := -1
	for rows.Next() {
		var r, c int
		var v sql.NullString
		if err := rows.Scan(&r, &c, &v); err != nil {
			return nil, err
		}
		for curRow < r { // 换行（curRow 从 -1 起，首行不额外换行）
			if curRow >= 0 {
				w.WriteByte('\n')
			}
			curRow++
		}
		if c > 0 {
			w.WriteByte(',')
		}
		if v.Valid {
			writeField(w, v.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if curRow >= 0 { // 末行结束换行；再导入时末尾单个换行不产生新行
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeField 按 CSV 规则写出一个单元格。
func writeField(w *bufio.Writer, s string) {
	if s == "" {
		w.WriteString(`""`) // 空串必须加引号，否则再导入时变成缺失值
		return
	}
	if !strings.ContainsAny(s, ",\"\r\n") {
		w.WriteString(s)
		return
	}
	w.WriteByte('"')
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			w.WriteByte('"')
		}
		w.WriteByte(s[i])
	}
	w.WriteByte('"')
}
