package string

import "testing"

func TestTruncateRune(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		maxRuneCount int
		want         string
	}{
		{
			name:         "ascii truncate",
			input:        "1234567890",
			maxRuneCount: 5,
			want:         "12345",
		},
		{
			name:         "ascii no truncate",
			input:        "hello",
			maxRuneCount: 10,
			want:         "hello",
		},
		{
			name:         "max rune count is zero",
			input:        "hello",
			maxRuneCount: 0,
			want:         "",
		},
		{
			name:         "max rune count is negative",
			input:        "hello",
			maxRuneCount: -1,
			want:         "",
		},
		{
			name:         "utf8 chinese",
			input:        "你好世界现在几点了",
			maxRuneCount: 4,
			want:         "你好世界",
		},
		{
			name:         "utf8 japanese",
			input:        "こんにちは世界",
			maxRuneCount: 5,
			want:         "こんにちは",
		},
		{
			name:         "utf8 korean",
			input:        "안녕하세요세계",
			maxRuneCount: 4,
			want:         "안녕하세",
		},
		{
			name:         "utf8 arabic",
			input:        "مرحبا بالعالم",
			maxRuneCount: 5,
			want:         "مرحبا",
		},
		{
			name:         "utf8 russian",
			input:        "Приветмир",
			maxRuneCount: 6,
			want:         "Привет",
		},
		{
			name:         "utf8 greek",
			input:        "γειασουκοσμε",
			maxRuneCount: 4,
			want:         "γεια",
		},
		{
			name:         "utf8 hebrew",
			input:        "שלוםעולם",
			maxRuneCount: 4,
			want:         "שלום",
		},
		{
			name:         "utf8 hindi",
			input:        "नमस्तेदुनिया",
			maxRuneCount: 6,
			want:         "नमस्ते",
		},
		{
			name:         "utf8 thai",
			input:        "กขคงจฉชซฌญ",
			maxRuneCount: 5,
			want:         "กขคงจ",
		},
		{
			name:         "utf8 vietnamese",
			input:        "ViệtNamXinChào",
			maxRuneCount: 4,
			want:         "Việt",
		},
		{
			name:         "utf8 spanish",
			input:        "EspañolMañana",
			maxRuneCount: 7,
			want:         "Español",
		},
		{
			name:         "utf8 portuguese",
			input:        "SãoToméPríncipe",
			maxRuneCount: 7,
			want:         "SãoTomé",
		},
		{
			name:         "utf8 german",
			input:        "Fußgängerstraße",
			maxRuneCount: 8,
			want:         "Fußgänge",
		},
		{
			name:         "utf8 turkish",
			input:        "İstanbulışık",
			maxRuneCount: 8,
			want:         "İstanbul",
		},
		{
			name:         "utf8 polish",
			input:        "Zażółćgęślą",
			maxRuneCount: 6,
			want:         "Zażółć",
		},
		{
			name:         "utf8 bengali",
			input:        "বাংলাভাষা",
			maxRuneCount: 7,
			want:         "বাংলাভা",
		},
		{
			name:         "utf8 tamil",
			input:        "தமிழ்மொழி",
			maxRuneCount: 5,
			want:         "தமிழ்",
		},
		{
			name:         "utf8 burmese script",
			input:        "ကခဂငစဆဇဈ",
			maxRuneCount: 5,
			want:         "ကခဂငစ",
		},
		{
			name:         "utf8 emoji",
			input:        "😀😃😄😁😆",
			maxRuneCount: 3,
			want:         "😀😃😄",
		},
		{
			name:         "mixed languages cjk thai vietnamese arabic emoji",
			input:        "Hello世界ไทยViệt😀مرحبا",
			maxRuneCount: 10,
			want:         "Hello世界ไทย",
		},
		{
			name:         "mixed languages cjk cyrillic arabic",
			input:        "Go语言Русскийالعربية",
			maxRuneCount: 9,
			want:         "Go语言Русск",
		},
		{
			name:         "utf8 no truncate",
			input:        "γειά",
			maxRuneCount: 10,
			want:         "γειά",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateRune(tt.input, tt.maxRuneCount)
			if got != tt.want {
				t.Fatalf("TruncateRune(%q, %d) = %q, want %q", tt.input, tt.maxRuneCount, got, tt.want)
			}
		})
	}
}