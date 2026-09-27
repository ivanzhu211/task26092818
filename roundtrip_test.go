package dataset

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// 随机文本单元格内容：混入逗号、引号、回车、换行、首尾空格、空字符串、
// 以及长得像数字/日期但按规则应判为文本的写法。
var nastyTexts = []string{
	"", " ", "  ", "lead", "trail ", " both ",
	"a,b", `say "hi"`, `"quoted"`, `a""b`,
	"line1\nline2", "car\rret", "crlf\r\nboth", "\n", "\r",
	"007", "1 ", " 1", "+1", "1,000", "1.5.6", "NaN", "Inf", "0x10",
	"1e5", "2/30/2020", "13/1/2023", "1/1/99", "2023/1/1",
	"中文内容", "café", "emoji🎉", "tab\there",
	"9223372036854775808", "1.0e400",
	`multi
line	with
specials`, `","`, `"""`,
}

// 保证判为文本的值，用来确保文本列不会被误推成其他类型
var guaranteedText = []string{"abc", "x", "hello world", "n/a", "中文"}

type colKind int

const (
	kindInt colKind = iota
	kindFloat
	kindDate
	kindText
)

func randomIntValue(rng *rand.Rand) string {
	switch rng.Intn(10) {
	case 0:
		return "0"
	case 1:
		return "-0"
	case 2:
		return "9223372036854775807"
	case 3:
		return "-9223372036854775808"
	default:
		v := rng.Int63()
		if rng.Intn(2) == 0 {
			v = -v
		}
		return fmt.Sprintf("%d", v)
	}
}

func randomFloatValue(rng *rand.Rand) string {
	switch rng.Intn(12) {
	case 0:
		return "1.7976931348623157e308" // 接近 float64 上限
	case 1:
		return "-1.7976931348623157e308"
	case 2:
		return "5.0e-324" // 最小正 denormal
	case 3:
		return "0.0"
	case 4:
		return "-0.0"
	default:
		intPart := fmt.Sprintf("%d", rng.Intn(100000))
		frac := fmt.Sprintf("%05d", rng.Intn(100000))
		frac = frac[:1+rng.Intn(5)]
		s := intPart + "." + frac
		if rng.Intn(3) == 0 { // 整数写法的小数，如 2.0
			s = intPart + ".0"
		}
		if rng.Intn(2) == 0 {
			s = "-" + s
		}
		if rng.Intn(3) == 0 { // 带指数
			e := rng.Intn(600) - 320
			s += fmt.Sprintf("e%+d", e)
		}
		return s
	}
}

func randomDateValue(rng *rand.Rand) string {
	switch rng.Intn(10) {
	case 0:
		return "2/29/2020" // 闰日
	case 1:
		return "2/29/2000"
	default:
		y := 1900 + rng.Intn(201)
		m := 1 + rng.Intn(12)
		d := 1 + rng.Intn(daysInMonth(m, y))
		if rng.Intn(2) == 0 {
			return fmt.Sprintf("%d/%d/%04d", m, d, y) // 一位写法
		}
		return fmt.Sprintf("%02d/%02d/%04d", m, d, y) // 两位写法
	}
}

func randomTextValue(rng *rand.Rand) string {
	return nastyTexts[rng.Intn(len(nastyTexts))]
}

// randomDataset 生成一个随机数据集，返回表头、记录和每列期望的类型。
func randomDataset(rng *rand.Rand) ([]string, []record, []ColType) {
	ncols := 1 + rng.Intn(6) // 包含只有一列的情况
	if rng.Intn(4) == 0 {
		ncols = 1
	}
	nrows := rng.Intn(50)

	header := make([]string, ncols)
	kinds := make([]colKind, ncols)
	wantTypes := make([]ColType, ncols)
	for c := 0; c < ncols; c++ {
		header[c] = fmt.Sprintf("col%d", c)
		kinds[c] = colKind(rng.Intn(4))
		switch kinds[c] {
		case kindInt:
			wantTypes[c] = ColInt
		case kindFloat:
			wantTypes[c] = ColFloat
		case kindDate:
			wantTypes[c] = ColDate
		default:
			wantTypes[c] = ColText
		}
	}

	rows := make([]record, nrows)
	for r := 0; r < nrows; r++ {
		rec := make(record, ncols)
		for c := 0; c < ncols; c++ {
			if rng.Intn(7) == 0 {
				rec[c] = nil // 缺失值
				continue
			}
			var v string
			switch kinds[c] {
			case kindInt:
				v = randomIntValue(rng)
			case kindFloat:
				v = randomFloatValue(rng)
			case kindDate:
				v = randomDateValue(rng)
			default:
				v = randomTextValue(rng)
			}
			vv := v
			rec[c] = &vv
		}
		rows[r] = rec
	}

	// 保证每列（有非缺失值时）推断出的类型符合预期：
	// 数值/日期列的值按构造都合法；文本列补一个保证为文本的值。
	for c := 0; c < ncols; c++ {
		if kinds[c] != kindText {
			continue
		}
		if nrows == 0 {
			continue
		}
		r := rng.Intn(nrows)
		v := guaranteedText[rng.Intn(len(guaranteedText))]
		rows[r][c] = &v
	}
	// 整列缺失时类型是文本
	for c := 0; c < ncols; c++ {
		allMissing := true
		for r := 0; r < nrows; r++ {
			if rows[r][c] != nil {
				allMissing = false
				break
			}
		}
		if allMissing {
			wantTypes[c] = ColText
		}
	}
	return header, rows, wantTypes
}

func TestRandomRoundTrip(t *testing.T) {
	s := openTemp(t)
	rng := rand.New(rand.NewSource(20260928))
	for iter := 0; iter < 200; iter++ {
		header, rows, wantTypes := randomDataset(rng)

		var buf bytes.Buffer
		if err := writeCSV(&buf, header, rows); err != nil {
			t.Fatalf("iter %d: write csv: %v", iter, err)
		}
		sum1, err := s.ImportCSV(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("iter %d: import: %v\ncsv:\n%s", iter, err, buf.String())
		}
		if sum1.Rows != len(rows) {
			t.Fatalf("iter %d: row count %d, want %d", iter, sum1.Rows, len(rows))
		}
		for c, want := range wantTypes {
			if sum1.Columns[c].Type != want {
				t.Fatalf("iter %d: col %d type %v, want %v\ncsv:\n%s",
					iter, c, sum1.Columns[c].Type, want, buf.String())
			}
		}

		var exported bytes.Buffer
		if err := s.ExportCSV(&exported, sum1.ID); err != nil {
			t.Fatalf("iter %d: export: %v", iter, err)
		}
		sum2, err := s.ImportCSV(bytes.NewReader(exported.Bytes()))
		if err != nil {
			t.Fatalf("iter %d: re-import: %v\nexported:\n%s", iter, err, exported.String())
		}
		assertSameDataset(t, s, sum1.ID, sum2.ID)
	}
}
