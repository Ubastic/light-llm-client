package utils

import (
	"strings"
	"testing"
)

func TestAnonymizer_CustomHeaders(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试自定义 header
	text := `fetch('https://api.example.com/data', {
  headers: {
    'X-Custom-Auth-Token': 'cust_live_abc123XYZ789def456',
    'X-App-Secret': 'myapp_secret_key_2024_v1',
    'X-Middleware-Key': 'mw_prod_aBcDeFgHiJkLmNoPqRsTuVwXyZ123'
  }
})`

	anonymized := a.Anonymize(text)

	// 验证所有自定义 token 都被匿名化
	if strings.Contains(anonymized, "cust_live_abc123XYZ789def456") {
		t.Errorf("Custom auth token should be anonymized")
	}
	if strings.Contains(anonymized, "myapp_secret_key_2024_v1") {
		t.Errorf("App secret should be anonymized")
	}
	if strings.Contains(anonymized, "mw_prod_aBcDeFgHiJkLmNoPqRsTuVwXyZ123") {
		t.Errorf("Middleware key should be anonymized")
	}

	// 验证可以还原
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed.\nExpected:\n%s\n\nGot:\n%s", text, deanonymized)
	}
}

func TestAnonymizer_JSONStructure(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试 JSON 结构中的敏感数据
	text := `{
  "url": "https://api.service.com/v1/endpoint",
  "headers": {
    "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature",
    "X-API-Key": "test_live_1234567890abcdefghijklmnop",
    "X-Custom-Token": "custom_token_aBcDeF123456"
  },
  "body": {
    "user_email": "user@example.com",
    "api_secret": "secret_key_xyz789"
  }
}`

	anonymized := a.Anonymize(text)

	t.Logf("Mapping count after anonymize: %d", a.GetMappingCount())
	for k, v := range a.mapping {
		if strings.HasPrefix(k, "KV_") {
			t.Logf("KV mapping: %s -> %s", k, v)
		}
	}

	// 验证敏感字段被匿名化
	if strings.Contains(anonymized, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("JWT token should be anonymized")
	}
	if strings.Contains(anonymized, "test_key_1234567890abcdefghijklmnop") {
		t.Errorf("API key should be anonymized")
	}
	if strings.Contains(anonymized, "custom_token_aBcDeF123456") {
		t.Errorf("Custom token should be anonymized")
	}
	if strings.Contains(anonymized, "user@example.com") {
		t.Errorf("Email should be anonymized")
	}
	if strings.Contains(anonymized, "secret_key_xyz789") {
		t.Errorf("Secret key should be anonymized")
	}

	// 验证可以还原
	deanonymized := a.Deanonymize(anonymized)
	// JSON 可能格式化不同，但内容应该相同
	if !strings.Contains(deanonymized, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("JWT token should be restored")
	}
}

func TestAnonymizer_KeyValuePairs(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试各种 key-value 格式
	text := `Configuration:
  api_key: test_key_abcdefghijklmnopqrstuvwxyz
  secret_token: secret_1234567890_ABCDEF
  auth_header: Bearer custom_auth_token_xyz123
  x-custom-key=custom_value_aBcDeF123456789
  middleware_token=mw_token_XyZ789AbC123`

	anonymized := a.Anonymize(text)
	t.Logf("Mapping count after anonymize: %d", a.GetMappingCount())
	for k, v := range a.mapping {
		if strings.HasPrefix(k, "KV_") {
			t.Logf("KV mapping: %s -> %s", k, v)
			t.Logf("Contains key in anonymized: %v", strings.Contains(anonymized, k))
		}
	}

	// 验证所有敏感值被匿名化
	if strings.Contains(anonymized, "test_key_abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("API key should be anonymized")
	}
	if strings.Contains(anonymized, "secret_1234567890_ABCDEF") {
		t.Errorf("Secret token should be anonymized")
	}
	if strings.Contains(anonymized, "custom_auth_token_xyz123") {
		t.Errorf("Auth token should be anonymized")
	}
	if strings.Contains(anonymized, "custom_value_aBcDeF123456789") {
		t.Errorf("Custom value should be anonymized")
	}

	// 验证可以还原
	deanonymized := a.Deanonymize(anonymized)
	t.Logf("Deanonymized:\n%s", deanonymized)
	if !strings.Contains(deanonymized, "test_key_abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("API key should be restored")
	}
}

func TestAnonymizer_HighEntropyDetection(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试高熵字符串检测（可能是 token）
	text := `The token is: aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7bC9dE1fG3hI5jK7lM9nO1pQ3rS5tU7vW9xY1zA3
And another one: XyZ123AbC456DeF789GhI012JkL345MnO678PqR901StU234VwX567YzA890BcD123`

	anonymized := a.Anonymize(text)

	// 验证高熵字符串被匿名化
	if strings.Contains(anonymized, "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7bC9dE1fG3hI5jK7lM9nO1pQ3rS5tU7vW9xY1zA3") {
		t.Errorf("High entropy string should be anonymized")
	}
	if strings.Contains(anonymized, "XyZ123AbC456DeF789GhI012JkL345MnO678PqR901StU234VwX567YzA890BcD123") {
		t.Errorf("High entropy string should be anonymized")
	}

	// 验证可以还原
	deanonymized := a.Deanonymize(anonymized)
	if deanonymized != text {
		t.Errorf("Deanonymization failed")
	}
}

func TestAnonymizer_RealWorldFetchExample(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 真实场景：fetch 请求包含各种自定义 header 和 token
	text := `const response = await fetch('https://api.myservice.com/v2/data', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c',
    'X-API-Key': 'test_key_51234567890abcdefghijklmnopqrstuvwxyz',
    'X-Client-ID': 'client_prod_aBcDeF123456',
    'X-Request-ID': 'req_XyZ789AbC123DeF456',
    'X-Middleware-Token': 'mw_custom_token_2024_v1_aBcDeF',
    'X-App-Secret': 'app_secret_key_prod_xyz123ABC'
  },
  body: JSON.stringify({
    email: 'user@company.com',
    server_ip: '192.168.1.100',
    database_url: 'postgresql://admin:password123@db.internal.com:5432/mydb',
    api_endpoint: 'https://internal-api.company.com/webhook'
  })
});`

	anonymized := a.Anonymize(text)

	// 验证所有敏感信息都被匿名化
	sensitiveData := []string{
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"test_key_51234567890abcdefghijklmnopqrstuvwxyz",
		"client_prod_aBcDeF123456",
		"req_XyZ789AbC123DeF456",
		"mw_custom_token_2024_v1_aBcDeF",
		"app_secret_key_prod_xyz123ABC",
		"user@company.com",
		"192.168.1.100",
		"postgresql://admin:password123@db.internal.com:5432/mydb",
		"https://internal-api.company.com/webhook",
	}

	for _, data := range sensitiveData {
		if strings.Contains(anonymized, data) {
			t.Errorf("Sensitive data should be anonymized: %s", data)
		}
	}

	// 验证可以完整还原
	deanonymized := a.Deanonymize(anonymized)
	for _, data := range sensitiveData {
		if !strings.Contains(deanonymized, data) {
			t.Errorf("Sensitive data should be restored: %s", data)
		}
	}
}

func TestAnonymizer_MixedContent(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试混合内容：代码 + 配置 + 说明文字
	text := `我在使用 fetch 请求时遇到问题：

fetch('https://api.example.com/users', {
  headers: {
    'Authorization': 'Bearer my_custom_token_abc123XYZ',
    'X-Custom-Header': 'custom_value_aBcDeF123'
  }
})

配置文件内容：
api_key: test_key_1234567890abcdefghijklmn
secret: my_secret_key_xyz789ABC
endpoint: https://internal.company.com/api

请帮我看看哪里有问题。我的邮箱是 developer@company.com`

	anonymized := a.Anonymize(text)

	// 验证敏感信息被匿名化
	if strings.Contains(anonymized, "my_custom_token_abc123XYZ") {
		t.Errorf("Custom token should be anonymized")
	}
	if strings.Contains(anonymized, "test_key_1234567890abcdefghijklmn") {
		t.Errorf("API key should be anonymized")
	}
	if strings.Contains(anonymized, "developer@company.com") {
		t.Errorf("Email should be anonymized")
	}

	// 验证普通文字没有被破坏
	if !strings.Contains(anonymized, "我在使用 fetch 请求时遇到问题") {
		t.Errorf("Normal text should be preserved")
	}
	if !strings.Contains(anonymized, "请帮我看看哪里有问题") {
		t.Errorf("Normal text should be preserved")
	}

	// 验证可以还原
	deanonymized := a.Deanonymize(anonymized)
	if !strings.Contains(deanonymized, "my_custom_token_abc123XYZ") {
		t.Errorf("Token should be restored")
	}
}

func TestAnonymizer_SensitiveKeyDetection(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试各种敏感 key 的检测
	sensitiveKeys := []string{
		"api_key", "apikey", "api-key",
		"secret", "secret_key", "secret-token",
		"token", "access_token", "refresh_token",
		"auth", "authorization", "auth_token",
		"password", "passwd", "pwd",
		"credential", "credentials",
		"x-api-key", "x-auth-token", "x-access-token",
		"private_key", "signature", "certificate",
	}

	for _, key := range sensitiveKeys {
		if !a.isSensitiveKey(key) {
			t.Errorf("Key should be detected as sensitive: %s", key)
		}
	}

	// 测试非敏感 key
	normalKeys := []string{
		"name", "title", "description",
		"content", "message", "data",
		"user_id", "product_id",
	}

	for _, key := range normalKeys {
		if a.isSensitiveKey(key) {
			t.Errorf("Key should NOT be detected as sensitive: %s", key)
		}
	}
}

func TestAnonymizer_EntropyCalculation(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 高熵字符串（随机 token）
	highEntropyStrings := []string{
		"aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7",
		"XyZ123AbC456DeF789GhI012JkL345MnO678",
		"sk_live_aBcDeF123456XyZ789",
	}

	for _, s := range highEntropyStrings {
		entropy := a.calculateEntropy(s)
		if entropy < 3.0 {
			t.Errorf("String should have high entropy (>3.0): %s, got: %f", s, entropy)
		}
	}

	// 低熵字符串（重复模式）
	lowEntropyStrings := []string{
		"aaaaaaaaaa",
		"1111111111",
		"ababababab",
	}

	for _, s := range lowEntropyStrings {
		entropy := a.calculateEntropy(s)
		if entropy > 2.0 {
			t.Errorf("String should have low entropy (<2.0): %s, got: %f", s, entropy)
		}
	}
}
