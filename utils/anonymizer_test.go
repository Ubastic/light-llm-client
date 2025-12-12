package utils

import (
	"strings"
	"testing"
)

func TestAnonymizer_URLs(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "请访问 https://api.example.com/v1/users 获取数据"
	anonymized := a.Anonymize(text)
	
	if strings.Contains(anonymized, "api.example.com") {
		t.Errorf("URL should be anonymized, got: %s", anonymized)
	}
	
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed. Expected: %s, Got: %s", text, deanonymized)
	}
}

func TestAnonymizer_APIKeys(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "API Key: test_key_1234567890abcdefghij"
	anonymized := a.Anonymize(text)
	
	if strings.Contains(anonymized, "test_key_1234567890abcdefghij") {
		t.Errorf("API key should be anonymized, got: %s", anonymized)
	}
	
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed. Expected: %s, Got: %s", text, deanonymized)
	}
}

func TestAnonymizer_BearerToken(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	anonymized := a.Anonymize(text)
	
	if strings.Contains(anonymized, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("Bearer token should be anonymized, got: %s", anonymized)
	}
	
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed. Expected: %s, Got: %s", text, deanonymized)
	}
}

func TestAnonymizer_Email(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "请联系 user@example.com 获取帮助"
	anonymized := a.Anonymize(text)
	
	if strings.Contains(anonymized, "user@example.com") {
		t.Errorf("Email should be anonymized, got: %s", anonymized)
	}
	
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed. Expected: %s, Got: %s", text, deanonymized)
	}
}

func TestAnonymizer_IPAddress(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "服务器IP是 192.168.1.100"
	anonymized := a.Anonymize(text)
	
	if strings.Contains(anonymized, "192.168.1.100") {
		t.Errorf("IP address should be anonymized, got: %s", anonymized)
	}
	
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed. Expected: %s, Got: %s", text, deanonymized)
	}
}

func TestAnonymizer_FetchExample(t *testing.T) {
	a := NewAnonymizer(true)
	
	// Simulate a fetch request with URL, headers, and body
	text := `fetch('https://api.example.com/v1/data', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer test_live_1234567890abcdefghijklmnopqrstuvwxyz',
    'Content-Type': 'application/json',
    'X-API-Key': 'test_key_1234567890abcdefghij'
  },
  body: JSON.stringify({
    email: 'user@example.com',
    server: '10.0.0.50'
  })
})`
	
	anonymized := a.Anonymize(text)
	
	// Check that sensitive data is anonymized
	if strings.Contains(anonymized, "api.example.com") {
		t.Errorf("URL should be anonymized")
	}
	if strings.Contains(anonymized, "test_live_1234567890abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("Bearer token should be anonymized")
	}
	if strings.Contains(anonymized, "test_key_1234567890abcdefghij") {
		t.Errorf("API key should be anonymized")
	}
	if strings.Contains(anonymized, "user@example.com") {
		t.Errorf("Email should be anonymized")
	}
	if strings.Contains(anonymized, "10.0.0.50") {
		t.Errorf("IP address should be anonymized")
	}
	
	// Check that deanonymization restores original
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed.\nExpected:\n%s\n\nGot:\n%s", text, deanonymized)
	}
}

func TestAnonymizer_Disabled(t *testing.T) {
	a := NewAnonymizer(false)
	
	text := "API Key: test_key_1234567890abcdefghij, URL: https://api.example.com"
	anonymized := a.Anonymize(text)
	
	// When disabled, text should remain unchanged
	if anonymized != text {
		t.Errorf("When disabled, text should not be anonymized. Got: %s", anonymized)
	}
}

func TestAnonymizer_Clear(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "API Key: test_key_1234567890abcdefghij"
	anonymized := a.Anonymize(text)
	
	// Clear mappings
	a.Clear()
	
	// After clearing, deanonymization should not restore original
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized == text {
		t.Errorf("After clearing, deanonymization should not restore original")
	}
}

func TestAnonymizer_MultipleOccurrences(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "URL1: https://api.example.com, URL2: https://api.example.com"
	anonymized := a.Anonymize(text)
	
	// Both occurrences should be replaced with the same placeholder
	parts := strings.Split(anonymized, ", ")
	if len(parts) != 2 {
		t.Errorf("Expected 2 parts, got %d", len(parts))
	}
	
	// Extract the placeholder from both parts
	placeholder1 := strings.TrimPrefix(parts[0], "URL1: ")
	placeholder2 := strings.TrimPrefix(parts[1], "URL2: ")
	
	if placeholder1 != placeholder2 {
		t.Errorf("Same URL should have same placeholder. Got: %s and %s", placeholder1, placeholder2)
	}
	
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed. Expected: %s, Got: %s", text, deanonymized)
	}
}

func TestAnonymizer_WindowsPath(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "文件路径: C:\\Users\\Admin\\Documents\\secret.txt"
	anonymized := a.Anonymize(text)
	
	if strings.Contains(anonymized, "C:\\Users\\Admin") {
		t.Errorf("Windows path should be anonymized, got: %s", anonymized)
	}
	
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed. Expected: %s, Got: %s", text, deanonymized)
	}
}

func TestAnonymizer_DatabaseConnectionString(t *testing.T) {
	a := NewAnonymizer(true)
	
	text := "连接字符串: mongodb://user:password@localhost:27017/mydb"
	anonymized := a.Anonymize(text)
	
	if strings.Contains(anonymized, "mongodb://user:password@localhost") {
		t.Errorf("Database connection string should be anonymized, got: %s", anonymized)
	}
	
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed. Expected: %s, Got: %s", text, deanonymized)
	}
}
