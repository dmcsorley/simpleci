package config

import (
	"bufio"
	"errors"
	"github.com/hashicorp/hcl"
	"strings"
)

type Record struct {
	Values []*ValueRecord          `hcl:"value"`
	Builds map[string]*BuildRecord `hcl:"build"`
}

type ValueRecord struct {
	Name          string `hcl:",key"`
	Literal       string
	DecodedFields []string `hcl:",decodedFields"`
}

type BuildRecord struct {
	Steps []*StepRecord `hcl:"step,expand"`
}

type StepRecord struct {
	Type          string `hcl:",key"`
	Url           string
	Image         string
	Cmd           string
	Dir           string
	DecodedFields []string `hcl:",decodedFields"`
}

func hasField(fields []string, s string) bool {
	for _, f := range fields {
		if strings.EqualFold(s, f) {
			return true
		}
	}
	return false
}

func (vr *ValueRecord) HasField(s string) bool {
	return hasField(vr.DecodedFields, s)
}

func (sr *StepRecord) StepType() string {
	return sr.Type
}

func (sr *StepRecord) HasUrl() bool {
	return hasField(sr.DecodedFields, "url")
}

func (sr *StepRecord) UrlParam() string {
	return sr.Url
}

func (sr *StepRecord) HasImage() bool {
	return hasField(sr.DecodedFields, "image")
}

func (sr *StepRecord) ImageParam() string {
	return sr.Image
}

func (sr *StepRecord) HasDir() bool {
	return hasField(sr.DecodedFields, "dir")
}

func (sr *StepRecord) DirParam() string {
	return sr.Dir
}

func (sr *StepRecord) HasCmd() bool {
	return hasField(sr.DecodedFields, "cmd")
}

func (sr *StepRecord) CmdParam() string {
	return sr.Cmd
}

func (sr *StepRecord) Parameter(name string) string {
	switch strings.ToLower(name) {
	case "url":
		return sr.Url
	case "image":
		return sr.Image
	case "cmd":
		return sr.Cmd
	case "dir":
		return sr.Dir
	default:
		return ""
	}
}

func LoadBytes(b []byte) (*Record, error) {
	r := &Record{}
	err := hcl.Unmarshal(b, r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func tokenize(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		return 0, nil, nil
	}

	switch data[0] {
	case '$':
		return 1, []byte("$"), nil
	case '{':
		return 1, []byte("{"), nil
	case '}':
		return 1, []byte("}"), nil
	default:
		token = nil
	INNER:
		for _, b := range data {
			switch b {
			case '$', '{', '}':
				break INNER
			default:
				token = append(token, b)
			}
		}

		if len(token) < len(data) || atEOF {
			return len(token), token, nil
		} else {
			return 0, nil, nil
		}
	}
}

type ValueEngine struct {
	values map[string]string
}

func NewValueEngine() *ValueEngine {
	return &ValueEngine{values: map[string]string{}}
}

func (ve *ValueEngine) Add(name string, value string) {
	ve.values[name] = value
}

type parseState int

const (
	initial parseState = iota
	haveDollar
	haveLeftCurly
	haveValueName
)

func (ve *ValueEngine) Validate(astring string) error {
	s := bufio.NewScanner(strings.NewReader(astring))
	s.Split(tokenize)

	var valueName string
	state := initial

	for s.Scan() {
		t := s.Text()
		switch state {
		case initial:
			if t == "$" {
				state = haveDollar
			}
		case haveDollar:
			switch t {
			case "$":
				state = initial
			case "{":
				state = haveLeftCurly
			default:
				return errors.New("Unexpected '" + t + "' after $")
			}
		case haveLeftCurly:
			switch t {
			case "$", "{", "}":
				return errors.New("Unexpected '" + t + "' after {")
			default:
				state = haveValueName
				valueName = t
			}
		case haveValueName:
			switch t {
			case "}":
				if ve.values[valueName] == "" {
					return errors.New("Unexpected value '" + valueName + "'")
				}
				state = initial
			default:
				return errors.New("Unexpected '" + t + "' after '" + valueName + "'")
			}
		}
	}

	if state != initial {
		return errors.New("Unexpected end of stuff '" + astring + "'")
	}

	return nil
}

func (ve *ValueEngine) Replace(astring string) (string, error) {
	s := bufio.NewScanner(strings.NewReader(astring))
	s.Split(tokenize)

	var valueName string
	var result []byte
	state := initial

	for s.Scan() {
		t := s.Text()
		switch state {
		case initial:
			switch t {
			case "$":
				state = haveDollar
			default:
				result = append(result, []byte(t)...)
			}
		case haveDollar:
			switch t {
			case "$":
				result = append(result, '$')
				state = initial
			case "{":
				state = haveLeftCurly
			default:
				return "", errors.New("Unexpected token '" + t + "'")
			}
		case haveLeftCurly:
			switch t {
			case "}":
				return "", errors.New("Unexpected token '" + t + "'")
			default:
				valueName = t
				state = haveValueName
			}
		case haveValueName:
			switch t {
			case "}":
				result = append(result, []byte(ve.values[valueName])...)
				state = initial
			default:
				return "", errors.New("Unexpected token '" + t + "'")
			}
		}
	}

	return string(result), nil
}
