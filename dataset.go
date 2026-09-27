// Package dataset 提供数据集的 CSV 导入、类型识别、SQLite 存储与 CSV 导出。
// 作为库由上层数据故事平台调用，不包含接口层。
package dataset

// ColumnType 列类型。
type ColumnType int

const (
	TypeText  ColumnType = iota // 文本
	TypeInt                     // 整数
	TypeFloat                   // 小数
	TypeDate                    // 日期（月/日/四位年）
)

func (t ColumnType) String() string {
	switch t {
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeDate:
		return "date"
	default:
		return "text"
	}
}

// Cell 一个单元格。Null 为 true 表示缺失值；
// 否则 Value 为原始文本（空字符串表示带引号的空串 ""）。
type Cell struct {
	Null  bool
	Value string
}

// ColumnSummary 单列结构摘要。
type ColumnSummary struct {
	Name    string
	Type    ColumnType
	Missing int
}

// Dataset 数据集结构摘要（导入结果）。
type Dataset struct {
	ID      int64
	Name    string
	Columns []ColumnSummary
	Rows    int
}

// 规模上限（含边界，正好到上限可导入）。
const (
	MaxRows = 10000
	MaxCols = 50
)
