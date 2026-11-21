package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

//go:embed default_keywords.txt
var defaultKeywords string

// Keyword 는 keywords.txt 한 줄(카드 1개)입니다.
type Keyword struct {
	Side string // PRO(찬성) / CON(반대)
	Word string
	Desc string
}

var loadedKeywords []Keyword

// parseKeywords: 형식  찬성|반대 | 키워드 | 설명  ('#' 주석, 빈 줄 무시, PRO/CON 도 허용)
func parseKeywords(text string) ([]Keyword, error) {
	var out []Keyword
	seen := map[string]bool{}
	n := map[string]int{}
	for i, raw := range strings.Split(strings.TrimPrefix(text, "\xEF\xBB\xBF"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			return nil, fmt.Errorf("keywords.txt %d번째 줄: '찬성|반대 | 키워드 | 설명' 형식이어야 합니다", i+1)
		}
		var side string
		switch strings.ToUpper(strings.TrimSpace(parts[0])) {
		case "찬성", "PRO":
			side = "PRO"
		case "반대", "CON":
			side = "CON"
		default:
			return nil, fmt.Errorf("keywords.txt %d번째 줄: 맨 앞은 '찬성' 또는 '반대' 여야 합니다", i+1)
		}
		word := strings.TrimSpace(parts[1])
		if word == "" {
			return nil, fmt.Errorf("keywords.txt %d번째 줄: 키워드가 비어 있습니다", i+1)
		}
		if utf8.RuneCountInString(word) > 50 {
			return nil, fmt.Errorf("keywords.txt %d번째 줄: 키워드는 50자 이하여야 합니다 ('%s')", i+1, word)
		}
		if seen[side+word] {
			return nil, fmt.Errorf("keywords.txt %d번째 줄: '%s' 가 같은 편에 중복되었습니다", i+1, word)
		}
		seen[side+word] = true
		desc := ""
		if len(parts) > 2 {
			desc = strings.TrimSpace(strings.Join(parts[2:], "|"))
		}
		if desc == "" {
			desc = word
		}
		out = append(out, Keyword{Side: side, Word: word, Desc: desc})
		n[side]++
	}
	if n["PRO"] < 5 || n["CON"] < 5 {
		return nil, fmt.Errorf("keywords.txt: 찬성·반대 키워드가 각각 5개 이상 필요합니다 (현재 찬성 %d개, 반대 %d개)", n["PRO"], n["CON"])
	}
	return out, nil
}

// buildKeywordSQL: keyword_pool 표를 목록과 같게 채우고, situation_card 에 카드가 저장될 때
// 같은 방·같은 물품·같은 편에서 겹치지 않게 무작위 키워드로 바꿔 넣는 트리거를 만듭니다.
// (백엔드는 기존대로 카드를 저장하고, DB 가 저장 직전에 키워드/설명만 바꿉니다.)
func buildKeywordSQL(kws []Keyword) string {
	var b strings.Builder
	b.WriteString("CREATE TABLE IF NOT EXISTS keyword_pool (id INT AUTO_INCREMENT PRIMARY KEY, side VARCHAR(8) NOT NULL, " +
		"keyword VARCHAR(50) NOT NULL, content TEXT NOT NULL) DEFAULT CHARSET=utf8mb4;\n")
	b.WriteString("DELETE FROM keyword_pool;\n")
	for _, k := range kws {
		fmt.Fprintf(&b, "INSERT INTO keyword_pool (side, keyword, content) VALUES (%s, %s, %s);\n",
			sqlQuote(k.Side), sqlQuote(k.Word), sqlQuote(k.Desc))
	}
	b.WriteString("DROP TRIGGER IF EXISTS trg_situation_card_random_keyword;\n")
	b.WriteString("DELIMITER $$\n")
	b.WriteString(`CREATE TRIGGER trg_situation_card_random_keyword BEFORE INSERT ON situation_card
FOR EACH ROW
BEGIN
  DECLARE k VARCHAR(50) DEFAULT NULL;
  DECLARE c TEXT DEFAULT NULL;
  IF NEW.owner_role IN ('PRO', 'CON') THEN
    SELECT p.keyword, p.content INTO k, c
      FROM keyword_pool p
     WHERE p.side = NEW.owner_role
       AND p.keyword NOT IN (
             SELECT s.keyword FROM situation_card s
              WHERE s.room_id <=> NEW.room_id
                AND s.procurement_item_id <=> NEW.procurement_item_id
                AND s.owner_role = NEW.owner_role)
     ORDER BY RAND() LIMIT 1;
    IF k IS NOT NULL THEN
      SET NEW.keyword = k;
      SET NEW.content = c;
    END IF;
  END IF;
END$$
`)
	b.WriteString("DELIMITER ;\n")
	return b.String()
}

// applyKeywords: 키워드 반영은 실패해도 게임 실행은 계속합니다. (실패하면 카드 키워드는 기본 이름으로 나옵니다)
func applyKeywords(exec func(sql string) (string, error)) {
	if len(loadedKeywords) == 0 {
		return
	}
	if out, err := exec(buildKeywordSQL(loadedKeywords)); err != nil {
		fmt.Printf("\n  [주의] 키워드 카드 목록을 DB에 반영하지 못했습니다. (게임은 계속 실행됩니다. 카드 이름이 'PRO 키워드 1' 처럼 보일 수 있음)\n  %v\n  %s\n", err, strings.TrimSpace(out))
		return
	}
	fmt.Printf("  키워드 카드 %d개를 반영했습니다. 카드는 이 목록에서 무작위로 지급됩니다.\n", len(loadedKeywords))
}

// loadKeywords: exe 옆(또는 launcher/프로젝트 폴더)의 keywords.txt, 없으면 내장 기본 목록
func loadKeywords(root string) {
	text, src := defaultKeywords, "내장 기본 목록"
	for _, d := range []string{exeDir(), filepath.Join(root, "launcher"), root} {
		p := filepath.Join(d, "keywords.txt")
		if b, err := os.ReadFile(p); err == nil {
			text, src = string(b), p
			break
		}
	}
	kws, err := parseKeywords(text)
	if err != nil {
		fail("%v", err)
	}
	loadedKeywords = kws
	fmt.Printf("  키워드 카드 %d개를 읽었습니다. (%s)\n", len(kws), src)
}
