package main

import (
	"strings"
	"testing"
)

func TestParseItems(t *testing.T) {
	in := "\xEF\xBB\xBF# 주석\n\n전기버스 | 친환경 vs 비용 | 환경\n마스크\n드론 | 설명만\n"
	got, err := parseItems(in)
	if err != nil || len(got) != 3 {
		t.Fatalf("got %v err %v", got, err)
	}
	if got[0].Category != "환경" || got[1].Description != "마스크 관련 토론 쟁점" || got[2].Category != "기타" {
		t.Fatalf("bad parse: %+v", got)
	}
}

func TestParseItemsErrors(t *testing.T) {
	if _, err := parseItems("# 비었음\n"); err == nil {
		t.Fatal("빈 목록은 오류여야 함")
	}
	if _, err := parseItems("a\na\n"); err == nil {
		t.Fatal("중복은 오류여야 함")
	}
	if _, err := parseItems(" | 설명\n"); err == nil {
		t.Fatal("이름 없음은 오류여야 함")
	}
}

func TestSyncSQLEscapes(t *testing.T) {
	sql := buildSyncSQL([]Item{{Name: "O'Brien\\x", Category: "c", Description: "d'"}})
	if !strings.Contains(sql, `'O''Brien\\x'`) || !strings.Contains(sql, "NOT IN ('O''Brien") {
		t.Fatalf("escape 실패:\n%s", sql)
	}
	if !strings.Contains(sql, "current_item_id") || !strings.Contains(sql, "situation_card") {
		t.Fatal("참조 중인 물품 보호 조건 누락")
	}
}
