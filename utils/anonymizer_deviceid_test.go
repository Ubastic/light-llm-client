package utils

import (
	"strings"
	"testing"
)

func TestAnonymizer_DeviceIDFormats(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试你提供的真实 device_id 格式
	text := `{
  "device_id2": "dp1_G789fgdwrwrqwerfxCOu4456456YmqYPNOasdfsdrj/82tk+//2RdfgaG3JSDFh12==",
  "deviceid": "dp1_02gzx4gasewfsadfwSTh4Fl+N5byG5Xfzn6/fTFR7dyXMTY/JJUsTLfghfgfshgG9uxfGH07tEVSInIsdfgqt2T7//ZEo2346jV"
}`

	anonymized := a.Anonymize(text)

	t.Logf("Original:\n%s", text)
	t.Logf("Anonymized:\n%s", anonymized)

	// 验证两个 device_id 都被匿名化
	if strings.Contains(anonymized, "dp1_G789fgdwrwrqwerfxCOu4456456YmqYPNOasdfsdrj/82tk+//2RdfgaG3JSDFh12==") {
		t.Errorf("device_id2 should be anonymized")
	}
	if strings.Contains(anonymized, "dp1_02gzx4gasewfsadfwSTh4Fl+N5byG5Xfzn6/fTFR7dyXMTY/JJUsTLfghfgfshgG9uxfGH07tEVSInIsdfgqt2T7//ZEo2346jV") {
		t.Errorf("deviceid should be anonymized")
	}

	// 验证可以还原
	deanonymized := a.Deanonymize(anonymized)

	t.Logf("Deanonymized:\n%s", deanonymized)

	if !strings.Contains(deanonymized, "dp1_G789fgdwrwrqwerfxCOu4456456YmqYPNOasdfsdrj/82tk+//2RdfgaG3JSDFh12==") {
		t.Errorf("device_id2 should be restored")
	}
	if !strings.Contains(deanonymized, "dp1_02gzx4gasewfsadfwSTh4Fl+N5byG5Xfzn6/fTFR7dyXMTY/JJUsTLfghfgfshgG9uxfGH07tEVSInIsdfgqt2T7//ZEo2346jV") {
		t.Errorf("deviceid should be restored")
	}
}

func TestAnonymizer_VariousDeviceIDFormats(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试各种 device_id 格式
	testCases := []struct {
		name  string
		text  string
		check string
	}{
		{
			name:  "Base64 with prefix",
			text:  `device_id: "dp1_aBcDeF123456+/=XyZ789"`,
			check: "dp1_aBcDeF123456+/=XyZ789",
		},
		{
			name:  "Long alphanumeric with special chars",
			text:  `uuid: "550e8400-e29b-41d4-a716-446655440000"`,
			check: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:  "Custom format with underscores and slashes",
			text:  `machine_id: "mach_abc123_def456/ghi789/jkl012"`,
			check: "mach_abc123_def456/ghi789/jkl012",
		},
		{
			name:  "Base64 encoded data",
			text:  `fingerprint: "SGVsbG8gV29ybGQhIFRoaXMgaXMgYSB0ZXN0Lg=="`,
			check: "SGVsbG8gV29ybGQhIFRoaXMgaXMgYSB0ZXN0Lg==",
		},
		{
			name:  "Mixed format with dots and plus",
			text:  `client_id: "client.prod.v1+aBcDeF123456XyZ789"`,
			check: "client.prod.v1+aBcDeF123456XyZ789",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			anonymized := a.Anonymize(tc.text)

			if strings.Contains(anonymized, tc.check) {
				t.Errorf("Value should be anonymized: %s", tc.check)
			}

			deanonymized := a.Deanonymize(anonymized)
			if !strings.Contains(deanonymized, tc.check) {
				t.Errorf("Value should be restored: %s", tc.check)
			}

			// 清除映射以便下一个测试
			a.Clear()
		})
	}
}

func TestAnonymizer_ComplexRealWorldExample(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 真实场景：包含多种格式的敏感数据
	text := `Request Headers:
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N
X-Device-ID: dp1_G789fgdwrwrqwerfxCOu4456456YmqYPNOasdfsdrj/82tk+//2RdfgaG3JSDFh12==
X-Client-ID: client_prod_v2_aBcDeF123456
X-Request-ID: req_XyZ789AbC123DeF456GhI789

Request Body:
{
  "username": "user@example.com",
  "device_id": "dp1_02gzx4gasewfsadfwSTh4Fl+N5byG5Xfzn6/fTFR7dyXMTY/JJUsTLfghfgfshgG9uxfGH07tEVSInIsdfgqt2T7//ZEo2346jV",
  "fingerprint": "fp_aBcDeF123456XyZ789+/=",
  "api_key": "test_live_1234567890abcdefghijklmnopqrstuvwxyz",
  "session_token": "sess_aBcDeF123456XyZ789GhI012JkL345MnO678",
  "hardware_id": "hw_prod_aBcDeF123456/XyZ789/GhI012"
}`

	anonymized := a.Anonymize(text)

	// 验证所有敏感数据都被匿名化
	sensitiveData := []string{
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"dp1_G789fgdwrwrqwerfxCOu4456456YmqYPNOasdfsdrj/82tk+//2RdfgaG3JSDFh12==",
		"client_prod_v2_aBcDeF123456",
		"req_XyZ789AbC123DeF456GhI789",
		"user@example.com",
		"dp1_02gzx4gasewfsadfwSTh4Fl+N5byG5Xfzn6/fTFR7dyXMTY/JJUsTLfghfgfshgG9uxfGH07tEVSInIsdfgqt2T7//ZEo2346jV",
		"fp_aBcDeF123456XyZ789+/=",
		"test_live_1234567890abcdefghijklmnopqrstuvwxyz",
		"sess_aBcDeF123456XyZ789GhI012JkL345MnO678",
		"hw_prod_aBcDeF123456/XyZ789/GhI012",
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

func TestAnonymizer_SensitiveValueDetection(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试敏感值检测逻辑
	testCases := []struct {
		value    string
		expected bool
		reason   string
	}{
		{
			value:    "dp1_G789fgdwrwrqwerfxCOu4456456YmqYPNOasdfsdrj/82tk+//2RdfgaG3JSDFh12==",
			expected: true,
			reason:   "device_id with base64 encoding",
		},
		{
			value:    "aBcDeF123456XyZ789+/=",
			expected: true,
			reason:   "mixed case with digits and special chars",
		},
		{
			value:    "simple_text_value",
			expected: false,
			reason:   "simple lowercase text",
		},
		{
			value:    "12345678901234567890",
			expected: false,
			reason:   "only digits",
		},
		{
			value:    "abcdefghijklmnopqrst",
			expected: false,
			reason:   "only lowercase letters",
		},
		{
			value:    "ABCDEFGHIJKLMNOPQRST",
			expected: false,
			reason:   "only uppercase letters",
		},
		{
			value:    "aBcDeF123456",
			expected: true,
			reason:   "mixed case with digits (12+ chars)",
		},
		{
			value:    "test_123_abc",
			expected: true,
			reason:   "has digits, letters and underscores",
		},
		{
			value:    "prod/v1/abc123",
			expected: true,
			reason:   "has slashes, letters and digits",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.reason, func(t *testing.T) {
			result := a.looksLikeSensitiveValue(tc.value)
			if result != tc.expected {
				t.Errorf("Value: %s, Expected: %v, Got: %v, Reason: %s",
					tc.value, tc.expected, result, tc.reason)
			}
		})
	}
}

func TestAnonymizer_KeyDetection(t *testing.T) {
	a := NewAnonymizer(testPrivacyConfig(true))

	// 测试各种 key 的检测
	testCases := []struct {
		key      string
		expected bool
	}{
		{"device_id", true},
		{"deviceid", true},
		{"device_id2", true},
		{"hardware_id", true},
		{"fingerprint", true},
		{"uuid", true},
		{"guid", true},
		{"client_id", true},
		{"machine_id", true},
		{"session_id", true},
		{"api_key", true},
		{"secret_token", true},
		{"name", false},
		{"title", false},
		{"description", false},
		{"content", false},
		{"user_id", false},    // 普通业务 ID，不应该被检测为敏感
		{"product_id", false}, // 普通业务 ID，不应该被检测为敏感
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			result := a.isSensitiveKey(tc.key)
			if result != tc.expected {
				t.Errorf("Key: %s, Expected: %v, Got: %v", tc.key, tc.expected, result)
			}
		})
	}
}
