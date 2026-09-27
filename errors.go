package dataset

import "fmt"

// ErrCategory 是导入失败的原因类别。
type ErrCategory string

const (
	ErrNotUTF8      ErrCategory = "not_utf8"      // 不是合法 UTF-8
	ErrBadQuote     ErrCategory = "bad_quote"     // 引号使用不合法
	ErrBadLineEnd   ErrCategory = "bad_line_end"  // 非法行尾（孤立的 CR）
	ErrEmptyHeader  ErrCategory = "empty_header"  // 列名为空
	ErrDupHeader    ErrCategory = "dup_header"    // 列名重复
	ErrColumnCount  ErrCategory = "column_count"  // 列数与表头不一致
	ErrTooManyRows  ErrCategory = "too_many_rows" // 超过 10000 行数据
	ErrTooManyCols  ErrCategory = "too_many_cols" // 超过 50 列
	ErrEmptyDataset ErrCategory = "empty_dataset" // 文件没有任何记录
)

// ImportError 描述一次导入失败，Record 是出错位置的记录序号（表头为 0）。
type ImportError struct {
	Category ErrCategory
	Record   int
	Detail   string
}

func (e *ImportError) Error() string {
	return fmt.Sprintf("import failed: %s at record %d: %s", e.Category, e.Record, e.Detail)
}
