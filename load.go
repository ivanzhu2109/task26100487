package attrition

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFile 从本地 JSON 文件装载完整历史并构建只读 Store。
// 文件格式与 History 的 JSON 编码一致，日期为 "YYYY-MM-DD" 字符串。
func LoadFile(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("attrition: 读取历史文件失败: %w", err)
	}
	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("attrition: 解析历史文件失败: %w", err)
	}
	return Build(&h)
}

// SaveFile 将历史以 JSON 形式写入本地文件（便于生成可复现数据）。
func SaveFile(h *History, path string) error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("attrition: 编码历史失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("attrition: 写入历史文件失败: %w", err)
	}
	return nil
}
