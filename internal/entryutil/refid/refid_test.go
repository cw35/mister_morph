package refid

import "testing"

func TestParse(t *testing.T) {
	protocol, id, ok := Parse("SLACK:T123:C456")
	if !ok {
		t.Fatalf("Parse() should succeed")
	}
	if protocol != "slack" || id != "T123:C456" {
		t.Fatalf("Parse() mismatch: protocol=%q id=%q", protocol, id)
	}
}

func TestNormalize(t *testing.T) {
	out, ok := Normalize("TG:-1001981343441")
	if !ok {
		t.Fatalf("Normalize() should succeed")
	}
	if out != "tg:-1001981343441" {
		t.Fatalf("Normalize() mismatch: got %q", out)
	}
}

func TestIsValid(t *testing.T) {
	if !IsValid("peer:12D3KooWPeer") {
		t.Fatalf("IsValid(custom protocol) should be true")
	}
	if IsValid("peer-v2:12D3KooWPeer") {
		t.Fatalf("IsValid(protocol with punctuation) should be false")
	}
	if IsValid("not-a-reference") {
		t.Fatalf("IsValid(invalid) should be false")
	}
}

func TestParseTelegramChatIDHint(t *testing.T) {
	chatID, hasHint, err := ParseTelegramChatIDHint("tg:-1001981343441")
	if err != nil || !hasHint || chatID != -1001981343441 {
		t.Fatalf("ParseTelegramChatIDHint(tg) mismatch: chat_id=%d has_hint=%v err=%v", chatID, hasHint, err)
	}
	chatID, hasHint, err = ParseTelegramChatIDHint("12345")
	if err != nil || !hasHint || chatID != 12345 {
		t.Fatalf("ParseTelegramChatIDHint(raw) mismatch: chat_id=%d has_hint=%v err=%v", chatID, hasHint, err)
	}
	_, hasHint, err = ParseTelegramChatIDHint("")
	if err != nil || hasHint {
		t.Fatalf("ParseTelegramChatIDHint(empty) mismatch: has_hint=%v err=%v", hasHint, err)
	}
	_, _, err = ParseTelegramChatIDHint("slack:T001:C002")
	if err == nil {
		t.Fatalf("ParseTelegramChatIDHint(non tg) expected error")
	}
}

func TestParseSlackChatIDHint(t *testing.T) {
	teamID, channelID, hasHint, err := ParseSlackChatIDHint("SLACK:T001:C002")
	if err != nil || !hasHint || teamID != "T001" || channelID != "C002" {
		t.Fatalf("ParseSlackChatIDHint(valid) mismatch: team=%q channel=%q has_hint=%v err=%v", teamID, channelID, hasHint, err)
	}
	_, _, hasHint, err = ParseSlackChatIDHint("tg:1001")
	if err != nil || hasHint {
		t.Fatalf("ParseSlackChatIDHint(non slack) mismatch: has_hint=%v err=%v", hasHint, err)
	}
	_, _, hasHint, err = ParseSlackChatIDHint("slack:T001")
	if err == nil || !hasHint {
		t.Fatalf("ParseSlackChatIDHint(invalid slack) expected has_hint=true error")
	}
}
