package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	// 新增 log 以便調試
)

// JsonNullString 是一個 sql.NullString 的包裝類型，用於自訂 JSON (un)marshalling。
type JsonNullString struct {
	sql.NullString
}

// MarshalJSON 為 JsonNullString 實現 json.Marshaler 介面。
func (jns JsonNullString) MarshalJSON() ([]byte, error) {
	if !jns.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(jns.String)
}

// UnmarshalJSON 為 JsonNullString 實現 json.Unmarshaler 介面。
func (jns *JsonNullString) UnmarshalJSON(data []byte) error {
	// log.Printf("DEBUG: JsonNullString.UnmarshalJSON called with data: %s", string(data)) // 調試日誌
	// 如果 data 是 JSON "null"
	if string(data) == "null" {
		jns.String, jns.Valid = "", false
		// log.Println("DEBUG: JsonNullString unmarshalled as null.")
		return nil
	}
	// 嘗試將 data 作為一個標準的 JSON 字串來解析
	var s string
	// 這裡的 json.Unmarshal 是用來解析 data (它本身應該是一個 JSON 編碼的字串，例如 "\"hello\"" 或 "\"\"")
	// 到 Go 的 string 類型變數 s
	if err := json.Unmarshal(data, &s); err != nil {
		// 如果 data 不是一個有效的 JSON 字串 (例如 data 是 `{"key":"value"}` 或數字 `123`)
		// 這裡會報錯。我們的 Gemini 回應中，這些欄位應該是字串或 null。
		jns.String, jns.Valid = "", false
		// log.Printf("DEBUG: JsonNullString.UnmarshalJSON error unmarshalling to string: %v. Data: %s", err, string(data))
		return fmt.Errorf("JsonNullString: 期望 JSON 字串或 null，但得到 '%s': %w", string(data), err)
	}
	jns.String, jns.Valid = s, true
	// log.Printf("DEBUG: JsonNullString.UnmarshalJSON success: String='%s', Valid=%t", jns.String, jns.Valid)
	return nil
}

// 您可以為其他 sql.NullXXX 類型創建類似的包裝類型
