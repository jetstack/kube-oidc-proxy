package flags

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// This is a struct to implement the pflag.Value interface to introduce a
// map[string][]string flag type.
type stringToStringSliceValue struct {
	values *map[string][]string
}

var _ pflag.Value = &stringToStringSliceValue{}

// NewStringToStringSliceValue returns a pflag.Value interface that implements
// a flag that takes values into the map[string][]slice data structure.
func NewStringToStringSliceValue(p *map[string][]string) pflag.Value {
	return &stringToStringSliceValue{
		values: p,
	}
}

// This format is expecting a list of key value pairs, seperated by commas. A
// single index may have multiple entries.
// e.g.: a=-7,b=2,a=3
func (s *stringToStringSliceValue) Set(val string) error {
	if s.values == nil {
		m := make(map[string][]string)
		s.values = &m
	}

	if *s.values == nil {
		*s.values = make(map[string][]string)
	}

	if len(val) == 0 {
		return nil
	}

	var ss []string

	r := csv.NewReader(strings.NewReader(val))
	var err error
	ss, err = r.Read()
	if err != nil {
		*s.values = make(map[string][]string)
		return err
	}

	for _, pair := range ss {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			*s.values = make(map[string][]string)
			return fmt.Errorf("%s must be formatted as key=value", pair)
		}

		(*s.values)[kv[0]] = append((*s.values)[kv[0]], kv[1])
	}

	return nil
}

func (s *stringToStringSliceValue) Type() string {
	return "stringToStringSlice"
}

func (s *stringToStringSliceValue) String() string {
	var records []string
	for k, vs := range *s.values {
		for _, v := range vs {
			records = append(records, k+"="+v)
		}
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(records); err != nil {
		panic(err)
	}

	w.Flush()
	return "[" + strings.TrimSpace(buf.String()) + "]"
}
