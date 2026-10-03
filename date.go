package attrition

import (
	"fmt"
	"strconv"
)

// Date 表示公历自然日，不含时间与时区。零值表示“无日期”（开放区间端点）。
type Date struct {
	year  int
	month int
	day   int
}

// NewDate 由年月日构造 Date。
func NewDate(year, month, day int) Date {
	return Date{year: year, month: month, day: day}
}

// ParseDate 解析 "2006-01-02" 格式的日期字符串。
func ParseDate(s string) (Date, error) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return Date{}, fmt.Errorf("attrition: invalid date %q", s)
	}
	y, err := strconv.Atoi(s[0:4])
	if err != nil {
		return Date{}, fmt.Errorf("attrition: invalid date %q", s)
	}
	m, err := strconv.Atoi(s[5:7])
	if err != nil {
		return Date{}, fmt.Errorf("attrition: invalid date %q", s)
	}
	d, err := strconv.Atoi(s[8:10])
	if err != nil {
		return Date{}, fmt.Errorf("attrition: invalid date %q", s)
	}
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return Date{}, fmt.Errorf("attrition: invalid date %q", s)
	}
	return Date{year: y, month: m, day: d}, nil
}

// String 返回 "2006-01-02" 格式。
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day)
}

// IsZero 报告是否为零值。
func (d Date) IsZero() bool {
	return d.year == 0 && d.month == 0 && d.day == 0
}

// Year 返回年。
func (d Date) Year() int { return d.year }

// Month 返回月。
func (d Date) Month() int { return d.month }

// Day 返回日。
func (d Date) Day() int { return d.day }

// days 返回自 1970-01-01 以来的天数，用于区间运算（Howard Hinnant 算法）。
func (d Date) days() int {
	y := d.year
	m := d.month
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 && y%400 != 0 {
		era--
	}
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d.day - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// dateFromDays 是 days 的逆运算。
func dateFromDays(z int) Date {
	z += 719468
	era := z / 146097
	if z < 0 && z%146097 != 0 {
		era--
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp + 3
	if mp >= 10 {
		m = mp - 9
	}
	if m <= 2 {
		y++
	}
	return Date{year: y, month: m, day: d}
}

// Before 报告 d 是否早于 o。
func (d Date) Before(o Date) bool {
	return d.days() < o.days()
}

// After 报告 d 是否晚于 o。
func (d Date) After(o Date) bool {
	return d.days() > o.days()
}

// Equal 报告两个日期是否相同。
func (d Date) Equal(o Date) bool {
	return d.year == o.year && d.month == o.month && d.day == o.day
}

// AddDays 返回 d 之后 n 天的日期。
func (d Date) AddDays(n int) Date {
	return dateFromDays(d.days() + n)
}

// DaysUntil 返回从 d 到 o（不含）的自然日数。
func (d Date) DaysUntil(o Date) int {
	return o.days() - d.days()
}

// MarshalJSON 以 "2006-01-02" 字符串编码；零值编码为 null。
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(strconv.Quote(d.String())), nil
}

// UnmarshalJSON 解码 "2006-01-02" 字符串或 null。
func (d *Date) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*d = Date{}
		return nil
	}
	if len(b) < 2 || b[0] != '"' {
		return fmt.Errorf("attrition: invalid date JSON %s", string(b))
	}
	v, err := ParseDate(string(b[1 : len(b)-1]))
	if err != nil {
		return err
	}
	*d = v
	return nil
}
