package api_server

import "github.com/evgeniums/go-backend-helpers/pkg/api"

type EnumEntry struct {
	Value   string `json:"value"`
	Display string `json:"display"`
}

type EnumGetter = func(request Request) ([]*EnumEntry, error)

type DynamicTableField struct {
	Field   string       `json:"field"`
	Type    string       `json:"type"`
	Index   bool         `json:"index"`
	Display string       `json:"display"`
	Enum    []*EnumEntry `json:"enum,omitempty"`
}

type DynamicTable struct {
	api.ResponseStub
	Fields               []*DynamicTableField
	DefaultSortField     string `json:"default_sort_field"`
	DefaultSortDirection string `json:"default_sort_direction"`
}

type DynamicTableQuery struct {
	Path string `json:"path" validate:"required" vmessage:"Invalid table path."`
}

type DynamicTableConfig struct {
	Operation            api.Operation
	Model                interface{}
	Displays             map[string]string
	ColumnsOrder         []string
	DefaultSortField     string
	DefaultSortDirection string
	EnumGetter           EnumGetter
}

type DynamicFieldTranslator interface {
	Tr(field *DynamicTableField, tableName ...string)
}

type DynamicTables interface {
	AddTable(table *DynamicTableConfig)
	GetTable(path string) (*DynamicTable, error)
	SetTranslator(translator DynamicFieldTranslator)
}
