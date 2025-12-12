package main

import (
	"fmt"
	"light-llm-client/utils"
	"strings"
)

func main() {
	// 创建匿名化器
	anonymizer := utils.NewAnonymizer(true)
	
	fmt.Println("=== 匿名化演示 ===\n")
	
	// 示例 1: 包含 device_id 的 JSON
	example1 := `{
  "device_id2": "dp1_G789fgdwrwrqwerfxCOu4456456YmqYPNOasdfsdrj/82tk+//2RdfgaG3JSDFh12==",
  "deviceid": "dp1_02gzx4gasewfsadfwSTh4Fl+N5byG5Xfzn6/fTFR7dyXMTY/JJUsTLfghfgfshgG9uxfGH07tEVSInIsdfgqt2T7//ZEo2346jV"
}`
	
	fmt.Println("原始数据:")
	fmt.Println(example1)
	
	anonymized1 := anonymizer.Anonymize(example1)
	fmt.Println("\n匿名化后:")
	fmt.Println(anonymized1)
	
	deanonymized1 := anonymizer.Deanonymize(anonymized1)
	fmt.Println("\n还原后:")
	fmt.Println(deanonymized1)
	
	fmt.Println("\n" + strings.Repeat("=", 60) + "\n")
	
	// 清除映射
	anonymizer.Clear()
	
	// 示例 2: fetch 请求
	example2 := `fetch('https://api.example.com/v1/data', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature',
    'X-API-Key': 'test_live_1234567890abcdefghijklmnopqrstuvwxyz',
    'X-Device-ID': 'dp1_aBcDeF123456+/=XyZ789',
    'X-Custom-Token': 'custom_token_aBcDeF123456'
  },
  body: JSON.stringify({
    email: 'user@example.com',
    server: '192.168.1.100'
  })
})`
	
	fmt.Println("原始 fetch 请求:")
	fmt.Println(example2)
	
	anonymized2 := anonymizer.Anonymize(example2)
	fmt.Println("\n匿名化后:")
	fmt.Println(anonymized2)
	
	deanonymized2 := anonymizer.Deanonymize(anonymized2)
	fmt.Println("\n还原后:")
	fmt.Println(deanonymized2)
	
	fmt.Println("\n" + strings.Repeat("=", 60) + "\n")
	
	// 清除映射
	anonymizer.Clear()
	
	// 示例 3: 混合内容
	example3 := `我在调试一个 API 请求问题：

配置信息：
api_key: test_key_1234567890abcdefghijklmn
device_id: dp1_G789fgdwrwrqwerfxCOu4456456YmqYPNOasdfsdrj/82tk+//2RdfgaG3JSDFh12==
endpoint: https://api.internal.company.com/v2/webhook

请求代码：
fetch(endpoint, {
  headers: {
    'X-Custom-Auth': 'custom_auth_token_xyz123ABC',
    'X-Client-ID': 'client_prod_v2_aBcDeF123456'
  }
})

我的邮箱是 developer@company.com，请帮我看看哪里有问题。`
	
	fmt.Println("原始混合内容:")
	fmt.Println(example3)
	
	anonymized3 := anonymizer.Anonymize(example3)
	fmt.Println("\n匿名化后:")
	fmt.Println(anonymized3)
	
	deanonymized3 := anonymizer.Deanonymize(anonymized3)
	fmt.Println("\n还原后:")
	fmt.Println(deanonymized3)
	
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("演示完成！")
}
