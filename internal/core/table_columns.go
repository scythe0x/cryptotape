package core

import "errors"

type TableColumnID string

const (
	ColumnTime      TableColumnID = "time"
	ColumnTicker    TableColumnID = "ticker"
	ColumnEvent     TableColumnID = "event"
	ColumnTerminals TableColumnID = "terminals"
)

func DefaultTableColumns() []TableColumnID {
	return []TableColumnID{ColumnTime, ColumnTicker, ColumnEvent, ColumnTerminals}
}

func ValidateTableColumns(columns []TableColumnID) error {
	seen := make(map[TableColumnID]bool, len(columns))
	for _, column := range columns {
		switch column {
		case ColumnTime, ColumnTicker, ColumnEvent, ColumnTerminals:
		default:
			return errors.New("invalid table columns")
		}
		if seen[column] {
			return errors.New("invalid table columns")
		}
		seen[column] = true
	}
	return nil
}
