package config

import (
	"encoding/json"
	"schoolmailnotificator/definitions"
	"strings"
)

// генератор возвращает дефолтные значения и строит конфиг.
// генератор имеет метод Generate, возаращающий слепок конфига

func NewJsonConfigGenerator() *JsonConfigGenerator {
	return &JsonConfigGenerator{
		Root:   make(map[string]interface{}),
		prefix: "",
	}
}

type JsonConfigGenerator struct {
	Root   map[string]interface{}
	prefix string
}

func (j *JsonConfigGenerator) GetArray(path string) ([]definitions.Config, error) {
	panic("implement me")
}

func (j *JsonConfigGenerator) GetString(path string) (string, error) {
	if j.prefix != "" {
		path = j.prefix + jsonPathDelimiter + path
	}
	paths := strings.Split(path, jsonPathDelimiter)
	m := j.Root
	for _, p := range paths[:len(paths)-1] {
		_, ok := m[p]
		if !ok {
			m[p] = make(map[string]interface{})
		}
		m = m[p].(map[string]interface{})
	}
	s := "string"
	m[paths[len(paths)-1]] = &s
	return s, nil
}

func (j *JsonConfigGenerator) GetInt(path string) (int, error) {
	if j.prefix != "" {
		path = j.prefix + jsonPathDelimiter + path
	}
	paths := strings.Split(path, jsonPathDelimiter)
	m := j.Root
	for _, p := range paths[:len(paths)-1] {
		_, ok := m[p]
		if !ok {
			m[p] = make(map[string]interface{})
		}
		m = m[p].(map[string]interface{})
	}
	i := 1
	m[paths[len(paths)-1]] = &i
	return i, nil
}

func (j *JsonConfigGenerator) Child(path string) definitions.Config {
	if j.prefix != "" {
		path = j.prefix + jsonPathDelimiter + path
	}
	return &JsonConfigGenerator{
		Root:   j.Root,
		prefix: path,
	}
}

func (j *JsonConfigGenerator) Generate() ([]byte, error) {
	return json.MarshalIndent(j.Root, "", "\t")
}
