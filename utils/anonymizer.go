package utils

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// Anonymizer handles sensitive data anonymization and restoration
type Anonymizer struct {
	mu              sync.RWMutex
	mapping         map[string]string // anonymized -> original
	reverseMapping  map[string]string // original -> anonymized
	config          PrivacyConfig
	patterns        []AnonymizationPattern
	enabledPatterns map[string]bool
}

// AnonymizationPattern defines a pattern to detect and anonymize
type AnonymizationPattern struct {
	Name        string
	Regex       *regexp.Regexp
	Replacement string // Template for replacement, e.g., "URL_%d", "API_KEY_%d"
	Priority    int    // Higher priority patterns are processed first
}

// NewAnonymizer creates a new anonymizer with default patterns
func NewAnonymizer(config PrivacyConfig) *Anonymizer {
	a := &Anonymizer{
		mapping:        make(map[string]string),
		reverseMapping: make(map[string]string),
		config:         config,
	}
	
	// Initialize default patterns (ordered by priority)
	a.patterns = []AnonymizationPattern{
		// API Keys and Tokens (highest priority)
		{
			Name:        "Bearer Token",
			Regex:       regexp.MustCompile(`(?i)bearer\s+([a-zA-Z0-9_\-\.]{20,})`),
			Replacement: "BEARER_TOKEN_%s",
			Priority:    100,
		},
		{
			Name:        "API Key",
			Regex:       regexp.MustCompile(`(?i)(api[_-]?key|apikey|access[_-]?key|secret[_-]?key)[\s:=]+([a-zA-Z0-9_\-]{20,})`),
			Replacement: "API_KEY_%s",
			Priority:    95,
		},
		{
			Name:        "JWT Token",
			Regex:       regexp.MustCompile(`eyJ[a-zA-Z0-9_\-]+\.eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+`),
			Replacement: "JWT_TOKEN_%s",
			Priority:    90,
		},
		// 注释掉 Authorization Header 模式，因为它会与其他模式冲突
		// JWT Token 和 Bearer Token 模式会处理这些情况
		// {
		// 	Name:        "Authorization Header",
		// 	Regex:       regexp.MustCompile(`(?i)authorization[\s:]+(.+?)(?:\n|$|,)`),
		// 	Replacement: "AUTH_HEADER_%s",
		// 	Priority:    85,
		// },
		
		// URLs and Endpoints
		{
			Name:        "URL with Auth",
			Regex:       regexp.MustCompile(`https?://[^:]+:[^@]+@[^\s\)\"\']+`),
			Replacement: "URL_WITH_AUTH_%s",
			Priority:    80,
		},
		{
			Name:        "URL",
			Regex:       regexp.MustCompile(`https?://[^\s\)\"\'<>]+`),
			Replacement: "URL_%s",
			Priority:    75,
		},
		
		// Credentials
		{
			Name:        "Password",
			Regex:       regexp.MustCompile(`(?i)(password|passwd|pwd)[\s:=]+([^\s,\)\"\']+)`),
			Replacement: "PASSWORD_%s",
			Priority:    70,
		},
		{
			Name:        "Username",
			Regex:       regexp.MustCompile(`(?i)(username|user)[\s:=]+([^\s,\)\"\']+)`),
			Replacement: "USERNAME_%s",
			Priority:    65,
		},
		
		// Network Information
		{
			Name:        "IPv4 Address",
			Regex:       regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
			Replacement: "IP_ADDRESS_%s",
			Priority:    60,
		},
		{
			Name:        "IPv6 Address",
			Regex:       regexp.MustCompile(`(?i)\b(?:[0-9a-f]{1,4}:){7}[0-9a-f]{1,4}\b`),
			Replacement: "IPV6_ADDRESS_%s",
			Priority:    59,
		},
		{
			Name:        "MAC Address",
			Regex:       regexp.MustCompile(`(?i)\b(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b`),
			Replacement: "MAC_ADDRESS_%s",
			Priority:    58,
		},
		
		// Email and Contact
		{
			Name:        "Email",
			Regex:       regexp.MustCompile(`\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`),
			Replacement: "EMAIL_%s",
			Priority:    55,
		},
		{
			Name:        "Phone Number",
			Regex:       regexp.MustCompile(`(?:\+?86)?[-\s]?1[3-9]\d{9}\b`),
			Replacement: "PHONE_%s",
			Priority:    50,
		},
		
		// Database and Connection Strings
		{
			Name:        "Database Connection String",
			Regex:       regexp.MustCompile(`(?i)(mongodb|mysql|postgresql|redis|postgres)://[^\s\)\"\']+`),
			Replacement: "DB_CONNECTION_%s",
			Priority:    45,
		},
		
		// File Paths (Windows and Unix)
		{
			Name:        "Windows Path",
			Regex:       regexp.MustCompile(`[a-zA-Z]:\\(?:[^\s\)\"\'<>|*?]+\\)*[^\s\)\"\'<>|*?]+`),
			Replacement: "WIN_PATH_%s",
			Priority:    40,
		},
		{
			Name:        "Unix Path",
			Regex:       regexp.MustCompile(`/(?:home|root|usr|var|etc|opt)/[^\s\)\"\'<>]+`),
			Replacement: "UNIX_PATH_%s",
			Priority:    39,
		},
		
		// AWS and Cloud Credentials
		{
			Name:        "AWS Access Key",
			Regex:       regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`),
			Replacement: "AWS_ACCESS_KEY_%s",
			Priority:    35,
		},
		{
			Name:        "AWS Secret Key",
			Regex:       regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key[\s:=]+([a-zA-Z0-9/+=]{40})`),
			Replacement: "AWS_SECRET_KEY_%s",
			Priority:    34,
		},
		
		// Generic Secrets
		{
			Name:        "Generic Secret",
			Regex:       regexp.MustCompile(`(?i)(secret|token|key)[\s:=]+([a-zA-Z0-9_\-]{16,})`),
			Replacement: "SECRET_%s",
			Priority:    30,
		},
	}
	
	return a
}

// generatePlaceholder creates a consistent placeholder for a value
func (a *Anonymizer) generatePlaceholder(template, value string) string {
	// Use MD5 hash of the value to create a consistent but anonymized identifier
	hash := md5.Sum([]byte(value))
	hashStr := hex.EncodeToString(hash[:])[:8] // Use first 8 chars of hash
	return fmt.Sprintf(template, hashStr)
}

// Anonymize replaces sensitive information in the text
func (a *Anonymizer) Anonymize(text string) string {
	if !a.config.AnonymizeSensitiveData || text == "" {
		return text
	}
	
	a.mu.Lock()
	defer a.mu.Unlock()
	
	result := text
	
	// Step 1: Try to parse as JSON and anonymize structured data
	result = a.anonymizeJSON(result)
	
	// Step 2: Anonymize key-value pairs (headers, configs, etc.)
	result = a.anonymizeKeyValuePairs(result)
	
	// Step 3: Process regex patterns in priority order (highest first)
	for _, pattern := range a.patterns {
			if !a.isPatternEnabled(pattern) {
				continue
			}
		matches := pattern.Regex.FindAllStringSubmatch(result, -1)
		
		for _, match := range matches {
			if len(match) == 0 {
				continue
			}
			
			original := match[0]
			
			// Skip if already anonymized
			if _, exists := a.reverseMapping[original]; exists {
				continue
			}
			
			// Generate placeholder
			placeholder := a.generatePlaceholder(pattern.Replacement, original)
			
			// Store mapping
			a.mapping[placeholder] = original
			a.reverseMapping[original] = placeholder
			
			// Replace in text
			result = strings.ReplaceAll(result, original, placeholder)
		}
	}
	
	// Step 4: Anonymize high-entropy strings (likely tokens/keys)
	result = a.anonymizeHighEntropyStrings(result)
	
	return result
}

// Deanonymize restores original sensitive information in the text
func (a *Anonymizer) Deanonymize(text string) string {
	if !a.config.AnonymizeSensitiveData || text == "" {
		return text
	}
	
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	result := text
	
	// Replace all placeholders with original values
	for placeholder, original := range a.mapping {
		result = strings.ReplaceAll(result, placeholder, original)
	}
	
	return result
}

// Clear clears all stored mappings
func (a *Anonymizer) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.mapping = make(map[string]string)
	a.reverseMapping = make(map[string]string)
}

// SetEnabled enables or disables anonymization
func (a *Anonymizer) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.AnonymizeSensitiveData = enabled
}

// IsEnabled returns whether anonymization is enabled
func (a *Anonymizer) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config.AnonymizeSensitiveData
}

// GetMappingCount returns the number of anonymized values
func (a *Anonymizer) GetMappingCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.mapping)
}

// AddCustomPattern adds a custom anonymization pattern
func (a *Anonymizer) AddCustomPattern(name, regexPattern, replacement string, priority int) error {
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.patterns = append(a.patterns, AnonymizationPattern{
		Name:        name,
		Regex:       regex,
		Replacement: replacement,
		Priority:    priority,
	})
	
	return nil
}

// anonymizeJSON attempts to parse text as JSON and anonymize sensitive fields
func (a *Anonymizer) anonymizeJSON(text string) string {
	// Try to find JSON objects/arrays in the text
	var data interface{}
	
	// Try parsing the entire text as JSON
	if err := json.Unmarshal([]byte(text), &data); err == nil {
		// Successfully parsed as JSON
		anonymized := a.anonymizeJSONValue(data)
		if jsonBytes, err := json.Marshal(anonymized); err == nil {
			return string(jsonBytes)
		}
	}
	
	// Try to find embedded JSON objects
	result := text
	jsonObjRegex := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := jsonObjRegex.FindAllString(text, -1)
	
	for _, match := range matches {
		var obj interface{}
		if err := json.Unmarshal([]byte(match), &obj); err == nil {
			anonymized := a.anonymizeJSONValue(obj)
			if jsonBytes, err := json.Marshal(anonymized); err == nil {
				result = strings.Replace(result, match, string(jsonBytes), 1)
			}
		}
	}
	
	return result
}

// anonymizeJSONValue recursively anonymizes JSON values
func (a *Anonymizer) anonymizeJSONValue(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			lowerKey := strings.ToLower(key)
			
			// Check if key suggests sensitive data
			if a.isSensitiveKey(lowerKey) {
				// Anonymize the value
				if strVal, ok := value.(string); ok && strVal != "" {
					placeholder := a.anonymizeValue(strVal, "HEADER_"+strings.ToUpper(key))
					result[key] = placeholder
				} else {
					result[key] = a.anonymizeJSONValue(value)
				}
			} else {
				result[key] = a.anonymizeJSONValue(value)
			}
		}
		return result
		
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = a.anonymizeJSONValue(item)
		}
		return result
		
	case string:
		// Check if string looks like sensitive data
		if a.looksLikeSensitiveValue(v) {
			return a.anonymizeValue(v, "VALUE")
		}
		return v
		
	default:
		return v
	}
}

// isSensitiveKey checks if a key name suggests sensitive data
func (a *Anonymizer) isSensitiveKey(key string) bool {
	sensitiveKeywords := []string{
		"key", "token", "secret", "password", "passwd", "pwd",
		"auth", "authorization", "credential", "api_key", "apikey",
		"access_token", "refresh_token", "bearer", "session",
		"cookie", "x-api-key", "x-auth-token", "x-access-token",
		"private", "signature", "sign", "cert", "certificate",
		// 设备和标识符相关
		"device", "deviceid", "device_id", "uuid", "guid",
		"client_id", "clientid", 
		"machine_id", "machineid", "hardware_id", "hardwareid",
		"fingerprint", "identifier",
		// 其他可能的敏感字段
		"code", "nonce", "challenge", "hash",
	}
	
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(key, keyword) {
			return true
		}
	}
	
	return false
}

// anonymizeKeyValuePairs anonymizes key-value pairs in various formats
func (a *Anonymizer) anonymizeKeyValuePairs(text string) string {
	result := text
	
	// Pattern 1: "key": "value" (double quotes)
	kvRegex1 := regexp.MustCompile(`"([\w\-]+)"\s*:\s*"([^"]{10,})"`)
	matches := kvRegex1.FindAllStringSubmatch(result, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			key := match[1]
			value := match[2]
			if a.isSensitiveKey(strings.ToLower(key)) && a.looksLikeSensitiveValue(value) {
				placeholder := a.anonymizeValue(value, "KV_"+strings.ToUpper(key))
				result = strings.Replace(result, match[0], 
					`"`+key+`": "`+placeholder+`"`, 1)
			}
		}
	}
	
	// Pattern 1b: 'key': 'value' (single quotes)
	kvRegex1b := regexp.MustCompile(`'([\w\-]+)'\s*:\s*'([^']{10,})'`)
	matches = kvRegex1b.FindAllStringSubmatch(result, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			key := match[1]
			value := match[2]
			if a.isSensitiveKey(strings.ToLower(key)) && a.looksLikeSensitiveValue(value) {
				placeholder := a.anonymizeValue(value, "KV_"+strings.ToUpper(key))
				result = strings.Replace(result, match[0], 
					`'`+key+`': '`+placeholder+`'`, 1)
			}
		}
	}
	
	// Pattern 2: key: value (without quotes)
	kvRegex2 := regexp.MustCompile(`(?m)^[\s]*([a-zA-Z][\w\-]*)\s*:\s*([^\s,\n]{10,})`)
	matches = kvRegex2.FindAllStringSubmatch(result, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			key := match[1]
			value := match[2]
			if a.isSensitiveKey(strings.ToLower(key)) && a.looksLikeSensitiveValue(value) {
				placeholder := a.anonymizeValue(value, "KV_"+strings.ToUpper(key))
				result = strings.Replace(result, match[0], 
					strings.Replace(match[0], value, placeholder, 1), 1)
			}
		}
	}
	
	// Pattern 3: key=value
	kvRegex3 := regexp.MustCompile(`([a-zA-Z][\w\-]*)=([^\s&,;]{10,})`)
	matches = kvRegex3.FindAllStringSubmatch(result, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			key := match[1]
			value := match[2]
			if a.isSensitiveKey(strings.ToLower(key)) && a.looksLikeSensitiveValue(value) {
				placeholder := a.anonymizeValue(value, "KV_"+strings.ToUpper(key))
				result = strings.Replace(result, match[0], key+"="+placeholder, 1)
			}
		}
	}
	
	return result
}

// anonymizeHighEntropyStrings detects and anonymizes high-entropy strings (likely tokens)
func (a *Anonymizer) anonymizeHighEntropyStrings(text string) string {
	result := text
	
	// Find long alphanumeric strings with high entropy
	// Pattern: strings with 20+ chars containing mixed case, numbers, special chars
	// 支持更多特殊字符：下划线、连字符、点、斜杠、加号、等号
	highEntropyRegex := regexp.MustCompile(`[a-zA-Z0-9_\-\.\/+=]{20,}`)
	matches := highEntropyRegex.FindAllString(result, -1)
	
	for _, match := range matches {
		// Skip if already anonymized
		if _, exists := a.reverseMapping[match]; exists {
			continue
		}
		
		// 降低熵值要求，使用更智能的检测
		if a.looksLikeSensitiveValue(match) {
			placeholder := a.anonymizeValue(match, "HIGH_ENTROPY_TOKEN")
			result = strings.ReplaceAll(result, match, placeholder)
		}
	}
	
	return result
}

// calculateEntropy calculates Shannon entropy of a string
func (a *Anonymizer) calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	
	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}
	
	var entropy float64
	length := float64(len(s))
	
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * (float64(len(freq)) / length)
		}
	}
	
	return entropy
}

// looksLikeSensitiveValue checks if a value looks like sensitive data
func (a *Anonymizer) looksLikeSensitiveValue(value string) bool {
	if len(value) < 10 {
		return false
	}
	
	// Check for common patterns
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false
	specialCount := 0
	
	for _, c := range value {
		if unicode.IsUpper(c) {
			hasUpper = true
		} else if unicode.IsLower(c) {
			hasLower = true
		} else if unicode.IsDigit(c) {
			hasDigit = true
		} else if c == '_' || c == '-' || c == '.' || c == '/' || c == '+' || c == '=' {
			hasSpecial = true
			specialCount++
		}
	}
	
	// 计算字符种类数量
	varietyCount := 0
	if hasUpper {
		varietyCount++
	}
	if hasLower {
		varietyCount++
	}
	if hasDigit {
		varietyCount++
	}
	if hasSpecial {
		varietyCount++
	}
	
	// Likely a token/key if:
	// 1. Has 3+ types of characters (upper, lower, digit, special)
	// 2. Has mixed case and numbers
	// 3. Has digits and special chars (like device_id with base64)
	// 4. Has multiple special chars (like +, /, =)
	mixedCase := hasUpper && hasLower
	hasVariety := varietyCount >= 3 || 
		(mixedCase && hasDigit) || 
		(hasDigit && hasSpecial) || 
		(mixedCase && hasSpecial) ||
		specialCount >= 3
	
	// Also check entropy
	entropy := a.calculateEntropy(value)
	
	// 降低熵值阈值，因为 base64 编码的数据熵值可能不是特别高
	return hasVariety || entropy > 3.0
}

// anonymizeValue creates a placeholder for a sensitive value
func (a *Anonymizer) anonymizeValue(value, prefix string) string {
	// Skip if already anonymized (check if it's already a placeholder)
	if existing, exists := a.reverseMapping[value]; exists {
		return existing
	}
	
	// Check if this value is already a placeholder (to avoid double anonymization)
	if a.isPlaceholder(value) {
		return value
	}
	
	// Generate placeholder
	hash := md5.Sum([]byte(value))
	hashStr := hex.EncodeToString(hash[:])[:8]
	placeholder := fmt.Sprintf("%s_%s", prefix, hashStr)
	
	// Store mapping
	a.mapping[placeholder] = value
	a.reverseMapping[value] = placeholder
	
	return placeholder
}

// isPlaceholder checks if a string looks like an anonymization placeholder
// isPatternEnabled checks if a specific anonymization pattern is enabled by the config
func (a *Anonymizer) isPatternEnabled(pattern AnonymizationPattern) bool {
	switch pattern.Name {
	case "URL with Auth", "URL":
		return a.config.AnonymizeURLs
	case "Bearer Token", "API Key", "JWT Token", "AWS Access Key", "AWS Secret Key", "Generic Secret":
		return a.config.AnonymizeAPIKeys
	case "Email":
		return a.config.AnonymizeEmails
	case "IPv4 Address", "IPv6 Address":
		return a.config.AnonymizeIPAddresses
	case "Windows Path", "Unix Path":
		return a.config.AnonymizeFilePaths
	default:
		// For other general patterns like passwords, usernames, etc.,
		// we can tie them to the main AnonymizeSensitiveData flag.
		return a.config.AnonymizeSensitiveData
	}
}

func (a *Anonymizer) isPlaceholder(value string) bool {
	// Check if it's in our mapping
	if _, exists := a.mapping[value]; exists {
		return true
	}
	
	// Check if it matches placeholder pattern (PREFIX_hash)
	placeholderPattern := regexp.MustCompile(`^[A-Z_]+_[0-9a-f]{8}$`)
	return placeholderPattern.MatchString(value)
}
