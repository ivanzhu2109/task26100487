package attrition

import (
	"fmt"
	"time"
)

// Civil 由公历年月日构造 Date。m 取值 1-12。
func Civil(year, month, day int) Date {
	y := year
	m := month
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400            // [0, 399]
	mp := mod(m+9, 12)            // [0, 11]，3 月为 0
	doy := (153*mp+2)/5 + day - 1 // [0, 365]
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return Date(era*146097 + doe - 719468)
}

// Civil 返回 Date 对应的公历年月日。
func (d Date) Civil() (year, month, day int) {
	z := int(d) + 719468
	era := floorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	day = doy - (153*mp+2)/5 + 1
	month = mp + 3
	if mp >= 10 {
		month = mp - 9
	}
	year = y
	if month <= 2 {
		year = y + 1
	}
	return year, month, day
}

// ParseDate 解析 "YYYY-MM-DD" 或 RFC3339 日期部分。
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, fmt.Errorf("attrition: 无法解析日期 %q: %w", s, err)
	}
	return Civil(t.Year(), int(t.Month()), t.Day()), nil
}

// String 返回 "YYYY-MM-DD" 格式。
func (d Date) String() string {
	y, m, dd := d.Civil()
	return fmt.Sprintf("%04d-%02d-%02d", y, m, dd)
}

// Before 报告 d 是否早于 o。
func (d Date) Before(o Date) bool { return d < o }

// After 报告 d 是否晚于 o。
func (d Date) After(o Date) bool { return d > o }

// Days 返回 [d, o) 的自然日数（o 早于等于 d 时为非正数）。
func (d Date) DaysUntil(o Date) int { return int(o) - int(d) }

// Add 返回 n 天后的日期。
func (d Date) Add(n int) Date { return d + Date(n) }

// MarshalJSON 以 "YYYY-MM-DD" 字符串编码。
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON 解析 "YYYY-MM-DD" 字符串。
func (d *Date) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("attrition: 日期须为字符串: %s", b)
	}
	v, err := ParseDate(string(b[1 : len(b)-1]))
	if err != nil {
		return err
	}
	*d = v
	return nil
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func mod(a, b int) int {
	r := a % b
	if r < 0 {
		r += b
	}
	return r
}
