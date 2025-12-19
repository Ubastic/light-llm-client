package main

import (
	"fmt"
	"light-llm-client/utils"
)

func main() {
	config := utils.PrivacyConfig{
		AnonymizeSensitiveData: true,
		AnonymizeURLs:          true,
		AnonymizeAPIKeys:       true,
		AnonymizeEmails:        true,
		AnonymizeIPAddresses:   true,
		AnonymizeFilePaths:     true,
	}
	
	a := utils.NewAnonymizer(config)

	// Test 1: Simple JSON body
	text1 := `fetch('https://api.example.com/v1/data', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer test_live_1234567890abcdefghijklmnopqrstuvwxyz',
    'Content-Type': 'application/json',
    'X-API-Key': 'test_key_1234567890abcdefghij'
  },
  body: JSON.stringify({
    email: 'user@example.com',
    server: '10.0.0.50',
    name: 'John Doe',
    age: 25,
    deviceId: 'abc123def456ghi789'
  })
})`

	fmt.Println("========== 测试1：Fetch 请求 ==========")
	fmt.Println("\n原始文本:")
	fmt.Println(text1)
	fmt.Println("\n匿名化后:")
	anonymized1 := a.Anonymize(text1)
	fmt.Println(anonymized1)

	// Test 2: Direct JSON object
	text2 := `{
  "email": "user@example.com",
  "server": "10.0.0.50",
  "name": "John Doe",
  "age": 25,
  "deviceId": "abc123def456ghi789",
  "apiKey": "sk_test_1234567890abcdefghij",
  "description": "This is a test"
}`

	fmt.Println("\n\n========== 测试2：纯 JSON 对象 ==========")
	fmt.Println("\n原始文本:")
	fmt.Println(text2)
	fmt.Println("\n匿名化后:")
	anonymized2 := a.Anonymize(text2)
	fmt.Println(anonymized2)
	
	// Test 3: body with nested objects
	text3 := `{
  "body": {
    "user": {
      "email": "test@example.com",
      "name": "张三",
      "id": "user_abc123def456"
    },
    "settings": {
      "theme": "dark",
      "apiKey": "sk_prod_xyz789abc456"
    }
  }
}`

	fmt.Println("\n\n========== 测试3：嵌套 JSON 对象 ==========")
	fmt.Println("\n原始文本:")
	fmt.Println(text3)
	fmt.Println("\n匿名化后:")
	anonymized3 := a.Anonymize(text3)
	fmt.Println(anonymized3)
}
