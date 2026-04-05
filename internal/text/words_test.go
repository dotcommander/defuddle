package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"whitespace only", "   \t\n  ", 0},
		{"single word", "hello", 1},
		{"english sentence", "hello world test", 3},
		{"english with punctuation", "Hello, world! This is a test.", 6},
		{"chinese characters", "这是一个测试", 6},
		{"japanese hiragana", "これはテストです", 8},
		{"korean hangul", "이것은테스트입니다", 9},
		{"mixed CJK and latin", "Hello 世界 test", 4},
		{"CJK with spaces", "你好 世界", 4},
		{"leading trailing whitespace", "  hello world  ", 2},
		{"multiple spaces between words", "hello   world   test", 3},
		{"single CJK character", "字", 1},
		{"CJK followed by latin no space", "中文english", 3},
		{"latin followed by CJK no space", "english中文", 3},
		{"mixed paragraph", "The 东京 Olympics were held in 2021年", 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CountWords(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func BenchmarkCountWords(b *testing.B) {
	text := "Hello world this is a test with some 中文 characters 日本語 and Korean 한국어 mixed in for good measure."
	b.ReportAllocs()
	for range b.N {
		CountWords(text)
	}
}
