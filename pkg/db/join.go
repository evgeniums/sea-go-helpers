package db

import (
	"sync"

	"github.com/evgeniums/go-backend-helpers/pkg/logger"
	"github.com/evgeniums/go-backend-helpers/pkg/utils"
)

type JoinQuery interface {
	Join(ctx logger.WithLogger, filter *Filter, dest interface{}) (int64, error)
	Models() []interface{}
}

type Joiner interface {
	Join(model interface{}, field string, fieldsModel ...interface{}) JoinBegin
}

type JoinBegin interface {
	On(model interface{}, field string, fieldsModel ...interface{}) JoinEnd
}

type JoinEnd interface {
	Joiner
	Destination(dst interface{}) (JoinQuery, error)
}

type JoinTableData struct {
	Model       interface{}
	FieldsModel interface{}
}

type JoinTableBase struct {
	JoinTableData
}

func (j *JoinTableBase) Model() interface{} {
	return j.JoinTableData.Model
}

func (j *JoinTableBase) FieldsModel() interface{} {
	return j.JoinTableData.FieldsModel
}

type JoinPairData struct {
	LeftField  string
	RightField string
}

type JoinPairBase struct {
	JoinPairData
}

func (j *JoinPairBase) LeftField() string {
	return j.JoinPairData.LeftField
}

func (j *JoinPairBase) RightField() string {
	return j.JoinPairData.RightField
}

type JoinQueryData struct {
	destination interface{}
}

type JoinQueryBase struct {
	JoinQueryData
}

func (j *JoinQueryBase) Destination() interface{} {
	return j.JoinQueryData.destination
}

func (j *JoinQueryBase) SetDestination(destination interface{}) {
	j.JoinQueryData.destination = destination
}

type JoinQueryBuilder = func() (JoinQuery, error)

type JoinQueryConfig struct {
	Builder JoinQueryBuilder
	Name    string
	Nocache bool
}

func NewJoin(builder JoinQueryBuilder, name string, nocache ...bool) *JoinQueryConfig {
	return &JoinQueryConfig{Builder: builder, Name: name, Nocache: utils.OptionalArg(false, nocache...)}
}

type JoinQueries struct {
	mutex       sync.Mutex
	cache       map[string]JoinQuery
	modelsCache map[string][]interface{}
}

func NewJoinQueries() *JoinQueries {
	j := &JoinQueries{}
	j.cache = make(map[string]JoinQuery)
	j.modelsCache = make(map[string][]interface{})
	return j
}

func (j *JoinQueries) FindOrCreate(config *JoinQueryConfig) (JoinQuery, error) {
	var err error
	j.mutex.Lock()
	q, ok := j.cache[config.Name]
	j.mutex.Unlock()
	if !ok || config.Nocache {
		q, err = config.Builder()
		if err != nil {
			return nil, err
		}
	}
	j.mutex.Lock()
	if !config.Nocache {
		j.cache[config.Name] = q
	}
	j.modelsCache[config.Name] = q.Models()
	j.mutex.Unlock()
	return q, nil
}

func (j *JoinQueries) Models(config *JoinQueryConfig) ([]interface{}, error) {
	j.mutex.Lock()
	models, ok := j.modelsCache[config.Name]
	j.mutex.Unlock()
	if !ok {
		q, err := j.FindOrCreate(config)
		if err != nil {
			return nil, err
		}
		return q.Models(), nil
	}
	return models, nil
}
