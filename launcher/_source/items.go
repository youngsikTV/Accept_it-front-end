package main

import (
	"bufio"
	"fmt"
	"strings"
)

// Item 은 items.txt 한 줄(물품 1개)입니다.
type Item struct {
	Name        string
	Category    string
	Description string
}

// parseItems 는 items.txt 내용을 읽습니다.
// 한 줄 형식:  물품이름 | 설명 | 분류(선택)
// '#' 로 시작하는 줄과 빈 줄은 무시합니다.
func parseItems(text string) ([]Item, error) {
	var items []Item
	seen := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(text, "\xEF\xBB\xBF")))
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		parts := strings.Split(raw, "|")
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return nil, fmt.Errorf("items.txt %d번째 줄: 물품 이름이 비어 있습니다", line)
		}
		if seen[name] {
			return nil, fmt.Errorf("items.txt %d번째 줄: '%s' 가 중복되었습니다", line, name)
		}
		seen[name] = true
		it := Item{Name: name, Category: "기타"}
		if len(parts) > 1 {
			it.Description = strings.TrimSpace(parts[1])
		}
		if it.Description == "" {
			it.Description = name + " 관련 토론 쟁점" // DB 는 설명이 NOT NULL
		}
		if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
			it.Category = strings.TrimSpace(parts[2])
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("items.txt 에 물품이 한 개도 없습니다")
	}
	return items, nil
}

func sqlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `''`)
	return "'" + s + "'"
}

// buildSyncSQL 은 DB 의 procurement_item 을 items.txt 와 같은 내용으로 맞추는 SQL 을 만듭니다.
//  1. 목록에 있는 물품: 이미 있으면 설명/분류 갱신, 없으면 추가
//  2. 목록에 없는 물품: 지난 게임 기록(방/카드)이 참조하지 않는 것만 삭제
func buildSyncSQL(items []Item) string {
	var b strings.Builder
	b.WriteString("SET NAMES utf8mb4;\n")
	names := make([]string, 0, len(items))
	for _, it := range items {
		n := sqlQuote(it.Name)
		names = append(names, n)
		fmt.Fprintf(&b, "UPDATE procurement_item SET topic_category=%s, description=%s WHERE item_name=%s;\n",
			sqlQuote(it.Category), sqlQuote(it.Description), n)
		fmt.Fprintf(&b, "INSERT INTO procurement_item (item_name, topic_category, description) SELECT %s, %s, %s FROM DUAL "+
			"WHERE NOT EXISTS (SELECT 1 FROM procurement_item WHERE item_name=%s);\n",
			n, sqlQuote(it.Category), sqlQuote(it.Description), n)
	}
	fmt.Fprintf(&b, "DELETE FROM procurement_item WHERE item_name NOT IN (%s) "+
		"AND id NOT IN (SELECT current_item_id FROM room WHERE current_item_id IS NOT NULL) "+
		"AND id NOT IN (SELECT procurement_item_id FROM situation_card WHERE procurement_item_id IS NOT NULL);\n",
		strings.Join(names, ","))
	return b.String()
}
