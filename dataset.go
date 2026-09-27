// Package dataset 提供数据集的导入、类型识别、存储与导出能力。
// 数据集持久化在 SQLite 中，由上层数据故事平台以库的形式调用。
package dataset

// ColType 是列类型。
type ColType int

const (
	ColText  ColType = iota // 文本
	ColInt                  // 整数
	ColFloat                // 小数
	ColDate                 // 日期（月/日/四位年）
)

func (t ColType) String() string {
	switch t {
	case ColInt:
		return "int"
	case ColFloat:
		return "float"
	case ColDate:
		return "date"
	default:
		return "text"
	}
}

// Value 表示一个单元格的值。Null 为 true 时是缺失值。
type Value struct {
	Null  bool
	Int   int64
	Float float64
	Str   string // 文本值；日期按原始写法放在这里
}

// ColumnSummary 是单列的结构摘要。
type ColumnSummary struct {
	Name    string
	Type    ColType
	Missing int
}

// Summary 是数据集的结构摘要。
type Summary struct {
	ID      int64
	Rows    int
	Columns []ColumnSummary
}
