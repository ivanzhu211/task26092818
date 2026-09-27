package dataset

import "fmt"

// ErrCategory 导入失败的原因类别。
type ErrCategory string

const (
	CatEncoding ErrCategory = "encoding" // 非合法 UTF-8
	CatSyntax   ErrCategory = "syntax"   // CSV 词法/引号规则错误
	CatHeader   ErrCategory = "header"   // 表头为空或重复
	CatShape    ErrCategory = "shape"    // 行列数与表头不一致
	CatLimit    ErrCategory = "limit"    // 超出规模上限
)

// ImportError 导入错误，Record 为出错位置的记录序号
// （表头为 0，其后第一条数据记录为 1）。
type ImportError struct {
	Category ErrCategory
	Record   int
	Msg      string
}

func (e *ImportError) Error() string {
	return fmt.Sprintf("import: %s at record %d: %s", e.Category, e.Record, e.Msg)
}

func newErr(cat ErrCategory, record int, format string, args ...any) *ImportError {
	return &ImportError{Category: cat, Record: record, Msg: fmt.Sprintf(format, args...)}
}
