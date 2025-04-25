package util

import (
	"encoding/json"
	"strconv"
)

// 輔助函數：將 interface{} 轉換為 int
func ToInt(v interface{}) (int, bool) {
	switch value := v.(type) {
	case int:
		return value, true
	case float64:
		return int(value), true
	case json.Number:
		if i, err := value.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

// 輔助函數：將 interface{} 轉換為 float64
func ToFloat64(v interface{}) (float64, bool) {
	switch value := v.(type) {
	case float64:
		return value, true
	case int:
		return float64(value), true
	case json.Number:
		if f, err := value.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

func SafeFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// SafeInt 安全地將 interface{} 轉換為 int
func SafeInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case json.Number:
		i, err := val.Int64()
		return int(i), err == nil
	case string:
		i, err := strconv.Atoi(val)
		return i, err == nil
	default:
		return 0, false
	}
}

// SafeInt64 安全地將 interface{} 轉換為 int64
func SafeInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int64:
		return val, true
	case float64:
		return int64(val), true
	case json.Number:
		i, err := val.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(val, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}
