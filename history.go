package attrition

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// TenureSegment 是一段任职区间：Start（入职日）计为在岗，End（离职日）不计为在岗。
// Open 为 true 表示未离职，任职持续有效。
type TenureSegment struct {
	Employee string `json:"employee"`
	Start    Date   `json:"start"`
	End      Date   `json:"end,omitempty"`
	Open     bool   `json:"open,omitempty"`
}

// Assignment 是一段直接部门归属区间，[Start, End)，Open 表示持续有效。
type Assignment struct {
	Employee string `json:"employee"`
	Dept     string `json:"dept"`
	Start    Date   `json:"start"`
	End      Date   `json:"end,omitempty"`
	Open     bool   `json:"open,omitempty"`
}

// ParentChange 表示某部门自 Day 起（含当天）父部门变更为 NewParent。
type ParentChange struct {
	Dept      string `json:"dept"`
	NewParent string `json:"newParent"`
	Day       Date   `json:"day"`
}

// Dept 是部门的初始定义：Parent 为历史起点的父部门，空串表示根部门。
type Dept struct {
	ID     string `json:"id"`
	Parent string `json:"parent"`
}

// History 是完整历史数据的内存表示。
type History struct {
	Departments   []Dept          `json:"departments"`
	ParentChanges []ParentChange  `json:"parentChanges"`
	Tenures       []TenureSegment `json:"tenures"`
	Assignments   []Assignment    `json:"assignments"`
}

// LoadHistory 从本地 JSON 文件装载完整历史。
func LoadHistory(path string) (*History, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return DecodeHistory(f)
}

// DecodeHistory 从 io.Reader 解码 JSON 历史。
func DecodeHistory(r io.Reader) (*History, error) {
	var h History
	dec := json.NewDecoder(r)
	if err := dec.Decode(&h); err != nil {
		return nil, fmt.Errorf("attrition: decode history: %w", err)
	}
	return &h, nil
}

// Save 将历史以 JSON 写入本地文件。
func (h *History) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(h); err != nil {
		f.Close()
		return fmt.Errorf("attrition: encode history: %w", err)
	}
	return f.Close()
}
