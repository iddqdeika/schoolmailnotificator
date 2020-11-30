package config

import (
	"fmt"
	"os"
	"reflect"
	"schoolmailnotificator/definitions"
	"strconv"
	"strings"
)

const (
	argsPathDelimiter  = "."
	argsArrayDelimiter = ";"
)

func FromArgs() definitions.Config {
	argMap := make(map[string]string)
	for _, arg := range os.Args {
		i := strings.Index(arg, "=")
		if i < 0 {
			continue
		}
		name := arg[:i]
		val := arg[i+1:]
		argMap[name] = val
	}
	return &argsConfig{
		argMap: argMap,
	}
}

type argsConfig struct {
	pathPrefix string
	argMap     map[string]string
}

func (a *argsConfig) GetString(path string) (string, error) {
	var fp string
	if len(a.pathPrefix) > 0 {
		fp += a.pathPrefix + argsPathDelimiter
	}
	fp += path
	if val, ok := a.argMap[fp]; ok {
		return val, nil
	}
	return "", fmt.Errorf("cant get value for %v, it doesn't exist", fp)
}

func (a *argsConfig) GetInt(path string) (int, error) {
	sv, err := a.GetString(path)
	if err != nil {
		return 0, err
	}

	if val, err := strconv.Atoi(sv); err == nil {
		return val, nil
	}
	return 0, fmt.Errorf("value of %v is %v type and has unconvertable value %v", path, reflect.TypeOf(sv), sv)
}

func (a *argsConfig) GetArray(path string) ([]definitions.Config, error) {
	return nil, fmt.Errorf("array of structs not supported by args config")
}

func (a *argsConfig) Child(path string) definitions.Config {
	var fp string
	if len(a.pathPrefix) > 0 {
		fp += a.pathPrefix + argsPathDelimiter
	}
	fp += strings.Trim(path, argsPathDelimiter)
	return &argsConfig{
		pathPrefix: fp,
		argMap:     a.argMap,
	}
}
