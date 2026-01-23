package helpers

import (
	"testing"
)

func TestMD5Hash(t *testing.T) {
	// 测试用例
	tests := []struct {
		input    string
		expected string
	}{
		{"test", "d8578edf8458ce06fbc5bb76a58c5ca4"},
		{"hello world", "5eb63bbbe01eeed093cb22bb8f5acdc3"},
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MD5Hash(tt.input)
			if result != tt.expected {
				t.Errorf("MD5Hash(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestChineseToPinyin(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedHasCN  bool
		expectedOutput string
	}{
		{
			name:           "纯中文字符串",
			input:          "你好世界",
			expectedHasCN:  true,
			expectedOutput: "nhsj",
		},
		{
			name:           "纯英文字符串",
			input:          "Hello World",
			expectedHasCN:  false,
			expectedOutput: "Hello World",
		},
		{
			name:           "混合中英文",
			input:          "Hello 世界",
			expectedHasCN:  true,
			expectedOutput: "Hello sj",
		},
		{
			name:           "空字符串",
			input:          "",
			expectedHasCN:  false,
			expectedOutput: "",
		},
		{
			name:           "中文加数字",
			input:          "中国2024年",
			expectedHasCN:  true,
			expectedOutput: "zg2024n",
		},
		{
			name:           "电影标题示例",
			input:          "流浪地球2 (2023)",
			expectedHasCN:  true,
			expectedOutput: "lldq2 (2023)",
		},
		{
			name:           "电视剧标题示例",
			input:          "三体 The Three-Body Problem",
			expectedHasCN:  true,
			expectedOutput: "st The Three-Body Problem",
		},
		{
			name:           "包含标点符号",
			input:          "你好，世界！",
			expectedHasCN:  true,
			expectedOutput: "nh，sj！",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasCN, output := ChineseToPinyin(tt.input)
			if hasCN != tt.expectedHasCN {
				t.Errorf("ChineseToPinyin(%q) hasCN = %v; want %v", tt.input, hasCN, tt.expectedHasCN)
			}
			if output != tt.expectedOutput {
				t.Errorf("ChineseToPinyin(%q) output = %q; want %q", tt.input, output, tt.expectedOutput)
			}
			t.Logf("输入: %s -> 是否包含中文: %v, 输出: %s", tt.input, hasCN, output)
		})
	}
}
