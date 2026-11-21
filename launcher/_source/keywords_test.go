package main

import (
	"strings"
	"testing"
)

func TestDefaultKeywords(t *testing.T) {
	kws, err := parseKeywords(defaultKeywords)
	if err != nil {
		t.Fatal(err)
	}
	if len(kws) != 40 {
		t.Fatalf("기본 키워드 %d개 (40개 예상)", len(kws))
	}
}

func TestParseKeywordsErrors(t *testing.T) {
	few := "찬성|a|x\n반대|b|x\n"
	if _, err := parseKeywords(few); err == nil {
		t.Error("각 편 5개 미만인데 통과함")
	}
	if _, err := parseKeywords("중립|a|x\n"); err == nil {
		t.Error("잘못된 편이 통과함")
	}
	var b strings.Builder
	for i := 0; i < 5; i++ {
		b.WriteString("PRO|p" + string(rune('a'+i)) + "|x\nCON|c" + string(rune('a'+i)) + "|x\n")
	}
	b.WriteString("PRO|pa|dup\n")
	if _, err := parseKeywords(b.String()); err == nil {
		t.Error("중복이 통과함")
	}
}

func TestKeywordSQL(t *testing.T) {
	kws, _ := parseKeywords(defaultKeywords)
	kws[0].Desc = "it's a \\ test"
	sql := buildKeywordSQL(kws)
	for _, want := range []string{"CREATE TABLE IF NOT EXISTS keyword_pool", "DELIMITER $$", "BEFORE INSERT ON situation_card", "ORDER BY RAND()", "it''s a \\\\ test"} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 에 %q 없음", want)
		}
	}
	if strings.Count(sql, "INSERT INTO keyword_pool") != 40 {
		t.Error("INSERT 개수가 40이 아님")
	}
}
