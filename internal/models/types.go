package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	if string(data) == "null" {
		jns.String, jns.Valid = "", false
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		jns.String, jns.Valid = "", false
		return fmt.Errorf("JsonNullString: 期望 JSON 字串或 null，但得到 '%s': %w", string(data), err)
	}
	jns.String, jns.Valid = s, true
	return nil
}
