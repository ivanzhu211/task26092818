package dataset

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store 数据集存储，持久化于 SQLite。
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS datasets (
	id   INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	rows INTEGER NOT NULL,
	cols INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS columns (
	dataset_id INTEGER NOT NULL,
	idx        INTEGER NOT NULL,
	name       TEXT NOT NULL,
	type       INTEGER NOT NULL,
	missing    INTEGER NOT NULL,
	PRIMARY KEY (dataset_id, idx)
);
CREATE TABLE IF NOT EXISTS cells (
	dataset_id INTEGER NOT NULL,
	row        INTEGER NOT NULL,
	col        INTEGER NOT NULL,
	value      TEXT, -- NULL 表示缺失值；空串表示带引号的 ""
	PRIMARY KEY (dataset_id, row, col)
);
`

// Open 打开（必要时创建）位于 path 的 SQLite 存储。
// path 传 ":memory:" 可得纯内存存储。
func Open(path string) (*Store, error) {
	// 限制单连接，避免 :memory: 多连接各自为政，也省去并发写锁问题。
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	// 批量导入性能：事务仍保证原子性（失败整体回滚），
	// 仅放宽崩溃恢复级别的持久性保证。
	if _, err := db.Exec(`PRAGMA journal_mode = MEMORY; PRAGMA synchronous = OFF;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set pragmas: %w", err)
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

// Import 导入 CSV 字节流生成数据集，返回结构摘要。
// 格式违规时整份拒收，返回 *ImportError，不留下半个数据集。
func (s *Store) Import(name string, csvData []byte) (*Dataset, error) {
	records, perr := parseCSV(csvData)
	if perr != nil {
		return nil, perr
	}
	ds, cells, verr := buildDataset(records)
	if verr != nil {
		return nil, verr
	}
	ds.Name = name
	if err := s.insert(ds, cells); err != nil {
		return nil, err
	}
	return ds, nil
}

// buildDataset 校验记录结构并推断列类型，返回摘要与按行排列的单元格。
// 纯函数，不触碰存储。
func buildDataset(records [][]*string) (*Dataset, [][]*string, *ImportError) {
	if len(records) == 0 {
		return nil, nil, newErr(CatHeader, 0, "missing header row")
	}
	header := records[0]
	cols := len(header)
	if cols > MaxCols {
		return nil, nil, newErr(CatLimit, 0, "too many columns: %d > %d", cols, MaxCols)
	}
	// 表头：列名不能为空，也不能重复（区分大小写）。
	seen := make(map[string]bool, cols)
	names := make([]string, cols)
	for c, cell := range header {
		if cell == nil || *cell == "" {
			return nil, nil, newErr(CatHeader, 0, "empty column name at index %d", c)
		}
		if seen[*cell] {
			return nil, nil, newErr(CatHeader, 0, "duplicate column name %q", *cell)
		}
		seen[*cell] = true
		names[c] = *cell
	}
	rows := len(records) - 1
	if rows > MaxRows {
		return nil, nil, newErr(CatLimit, MaxRows+1, "too many rows: %d > %d", rows, MaxRows)
	}
	// 每行列数必须与表头相同。
	for r := 1; r < len(records); r++ {
		if len(records[r]) != cols {
			return nil, nil, newErr(CatShape, r,
				"record has %d fields, header has %d", len(records[r]), cols)
		}
	}
	// 按列收集单元格以推断类型、统计缺失。
	data := records[1:]
	summary := make([]ColumnSummary, cols)
	for c := 0; c < cols; c++ {
		colCells := make([]*string, rows)
		missing := 0
		for r := 0; r < rows; r++ {
			colCells[r] = data[r][c]
			if data[r][c] == nil {
				missing++
			}
		}
		summary[c] = ColumnSummary{
			Name:    names[c],
			Type:    inferColumnType(colCells),
			Missing: missing,
		}
	}
	return &Dataset{Columns: summary, Rows: rows}, data, nil
}

// insert 在单个事务内写入数据集、列与全部单元格，失败整体回滚。
func (s *Store) insert(ds *Dataset, data [][]*string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO datasets (name, rows, cols) VALUES (?, ?, ?)`,
		ds.Name, ds.Rows, len(ds.Columns))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	colStmt, err := tx.Prepare(
		`INSERT INTO columns (dataset_id, idx, name, type, missing) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer colStmt.Close()
	for i, c := range ds.Columns {
		if _, err := colStmt.Exec(id, i, c.Name, int(c.Type), c.Missing); err != nil {
			return err
		}
	}
	cellStmt, err := tx.Prepare(
		`INSERT INTO cells (dataset_id, row, col, value) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer cellStmt.Close()
	for r, row := range data {
		for c, cell := range row {
			var v any // nil → SQL NULL（缺失值）
			if cell != nil {
				v = *cell
			}
			if _, err := cellStmt.Exec(id, r, c, v); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	ds.ID = id
	return nil
}

// Summary 返回已有数据集的结构摘要。
func (s *Store) Summary(id int64) (*Dataset, error) {
	ds := &Dataset{ID: id}
	var cols int
	err := s.db.QueryRow(`SELECT name, rows, cols FROM datasets WHERE id = ?`, id).
		Scan(&ds.Name, &ds.Rows, &cols)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dataset %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT name, type, missing FROM columns WHERE dataset_id = ? ORDER BY idx`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cs ColumnSummary
		var t int
		if err := rows.Scan(&cs.Name, &t, &cs.Missing); err != nil {
			return nil, err
		}
		cs.Type = ColumnType(t)
		ds.Columns = append(ds.Columns, cs)
	}
	return ds, rows.Err()
}

// Row 按行号（0 起）查看一条记录的各单元格。
func (s *Store) Row(id int64, row int) ([]Cell, error) {
	if err := s.checkRow(id, row); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT value FROM cells WHERE dataset_id = ? AND row = ? ORDER BY col`, id, row)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cell
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, Cell{Null: !v.Valid, Value: v.String})
	}
	return out, rows.Err()
}

// Cell 按行号、列号（均 0 起）查看单个单元格。
func (s *Store) Cell(id int64, row, col int) (Cell, error) {
	if err := s.checkRow(id, row); err != nil {
		return Cell{}, err
	}
	var v sql.NullString
	err := s.db.QueryRow(
		`SELECT value FROM cells WHERE dataset_id = ? AND row = ? AND col = ?`,
		id, row, col).Scan(&v)
	if err == sql.ErrNoRows {
		return Cell{}, fmt.Errorf("column %d out of range", col)
	}
	if err != nil {
		return Cell{}, err
	}
	return Cell{Null: !v.Valid, Value: v.String}, nil
}

// checkRow 校验数据集存在且行号在范围内。
func (s *Store) checkRow(id int64, row int) error {
	var n int
	err := s.db.QueryRow(`SELECT rows FROM datasets WHERE id = ?`, id).Scan(&n)
	if err == sql.ErrNoRows {
		return fmt.Errorf("dataset %d not found", id)
	}
	if err != nil {
		return err
	}
	if row < 0 || row >= n {
		return fmt.Errorf("row %d out of range [0, %d)", row, n)
	}
	return nil
}
