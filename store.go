package dataset

import (
	"database/sql"
	"fmt"
	"io"
	"strconv"

	_ "modernc.org/sqlite"
)

// Store 管理存放在 SQLite 中的数据集。
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS datasets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	rows INTEGER NOT NULL,
	cols INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS columns (
	dataset_id INTEGER NOT NULL,
	idx INTEGER NOT NULL,
	name TEXT NOT NULL,
	type INTEGER NOT NULL,
	missing INTEGER NOT NULL,
	PRIMARY KEY (dataset_id, idx)
);
CREATE TABLE IF NOT EXISTS cells (
	dataset_id INTEGER NOT NULL,
	row INTEGER NOT NULL,
	col INTEGER NOT NULL,
	val TEXT,
	PRIMARY KEY (dataset_id, row, col)
);
`

// Open 打开（必要时创建）一个 SQLite 存储。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close 关闭存储。
func (s *Store) Close() error {
	return s.db.Close()
}

// ImportCSV 从 r 读入 CSV，校验并推断类型后存入 SQLite，返回结构摘要。
// 任何格式错误都会整份拒收，不留下半个数据集。
func (s *Store) ImportCSV(r io.Reader) (*Summary, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	recs, ierr := parseCSV(data)
	if ierr != nil {
		return nil, ierr
	}

	header := make([]string, len(recs[0]))
	for i, f := range recs[0] {
		header[i] = *f // 表头已校验非空
	}
	dataRows := recs[1:]

	// 逐列推断类型并统计缺失值
	cols := make([]ColumnSummary, len(header))
	for c := range header {
		var values []string
		missing := 0
		for _, rec := range dataRows {
			if rec[c] == nil {
				missing++
			} else {
				values = append(values, *rec[c])
			}
		}
		cols[c] = ColumnSummary{
			Name:    header[c],
			Type:    inferColumnType(values),
			Missing: missing,
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO datasets (rows, cols) VALUES (?, ?)`, len(dataRows), len(header))
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	colStmt, err := tx.Prepare(`INSERT INTO columns (dataset_id, idx, name, type, missing) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	for i, c := range cols {
		if _, err := colStmt.Exec(id, i, c.Name, int(c.Type), c.Missing); err != nil {
			colStmt.Close()
			return nil, err
		}
	}
	colStmt.Close()

	cellStmt, err := tx.Prepare(`INSERT INTO cells (dataset_id, row, col, val) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	for r, rec := range dataRows {
		for c, f := range rec {
			var val any
			if f != nil {
				val = *f
			}
			if _, err := cellStmt.Exec(id, r, c, val); err != nil {
				cellStmt.Close()
				return nil, err
			}
		}
	}
	cellStmt.Close()

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Summary{ID: id, Rows: len(dataRows), Columns: cols}, nil
}

// SummaryOf 返回已导入数据集的结构摘要。
func (s *Store) SummaryOf(id int64) (*Summary, error) {
	var rows, ncols int
	err := s.db.QueryRow(`SELECT rows, cols FROM datasets WHERE id = ?`, id).Scan(&rows, &ncols)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dataset %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	cols := make([]ColumnSummary, 0, ncols)
	r, err := s.db.Query(`SELECT name, type, missing FROM columns WHERE dataset_id = ? ORDER BY idx`, id)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for r.Next() {
		var c ColumnSummary
		var t int
		if err := r.Scan(&c.Name, &t, &c.Missing); err != nil {
			return nil, err
		}
		c.Type = ColType(t)
		cols = append(cols, c)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return &Summary{ID: id, Rows: rows, Columns: cols}, nil
}

// Cell 按行号（0 起，不含表头）和列下标读取单元格的值。
func (s *Store) Cell(id int64, row, col int) (Value, error) {
	var val *string
	err := s.db.QueryRow(
		`SELECT val FROM cells WHERE dataset_id = ? AND row = ? AND col = ?`, id, row, col,
	).Scan(&val)
	if err == sql.ErrNoRows {
		return Value{}, fmt.Errorf("no cell at row %d, col %d in dataset %d", row, col, id)
	}
	if err != nil {
		return Value{}, err
	}
	if val == nil {
		return Value{Null: true}, nil
	}
	var t int
	if err := s.db.QueryRow(
		`SELECT type FROM columns WHERE dataset_id = ? AND idx = ?`, id, col,
	).Scan(&t); err != nil {
		return Value{}, err
	}
	v := Value{Str: *val}
	switch ColType(t) {
	case ColInt:
		v.Int, _ = strconv.ParseInt(*val, 10, 64)
	case ColFloat:
		v.Float, _ = strconv.ParseFloat(*val, 64)
	}
	return v, nil
}

// ExportCSV 把数据集导出成 CSV 写入 w。
// 导出的文件再导入会得到完全相同的数据集。
func (s *Store) ExportCSV(w io.Writer, id int64) error {
	sum, err := s.SummaryOf(id)
	if err != nil {
		return err
	}
	header := make([]string, len(sum.Columns))
	for i, c := range sum.Columns {
		header[i] = c.Name
	}

	r, err := s.db.Query(
		`SELECT row, col, val FROM cells WHERE dataset_id = ? ORDER BY row, col`, id)
	if err != nil {
		return err
	}
	defer r.Close()

	rows := make([]record, 0, sum.Rows)
	for r.Next() {
		var rowIdx, colIdx int
		var val *string
		if err := r.Scan(&rowIdx, &colIdx, &val); err != nil {
			return err
		}
		for len(rows) <= rowIdx {
			rows = append(rows, make(record, len(header)))
		}
		rows[rowIdx][colIdx] = val
	}
	if err := r.Err(); err != nil {
		return err
	}
	return writeCSV(w, header, rows)
}
