package icon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

type SerializeError struct {
	position, msg string
}

func (s *SerializeError) Error() string {
	return s.position + ":" + s.msg
}

func serializeString(s string) []byte {
	ret := []byte(s)
	buf := new(bytes.Buffer)
	for _, b := range ret {
		switch b {
		case '\\':
			fallthrough
		case '{':
			fallthrough
		case '}':
			fallthrough
		case '[':
			fallthrough
		case ']':
			fallthrough
		case '.':
			buf.WriteByte('\\')
			buf.WriteByte(b)
		default:
			buf.WriteByte(b)
		}
	}
	return buf.Bytes()
}

func serializeList(v_list []interface{}) ([]byte, *SerializeError) {
	buf := new(bytes.Buffer)
	for idx, v := range v_list {
		frag, err := serializeValue(v)
		if err != nil {
			err.position = "[" + fmt.Sprint(idx) + "]." + err.position
			return nil, err
		}
		if buf.Len() > 0 {
			buf.WriteByte('.')
		}
		buf.Write(frag)
	}
	return buf.Bytes(), nil
}

func serializeValue(v interface{}) ([]byte, *SerializeError) {
	if v == nil {
		return []byte("\\0"), nil
	}
	v_dict, ok := v.(map[string]interface{})
	if ok {
		frag, err := serializeDict(v_dict, nil, nil)
		if err != nil {
			return nil, err
		}
		buf := new(bytes.Buffer)
		buf.WriteByte('{')
		buf.Write(frag)
		buf.WriteByte('}')
		return buf.Bytes(), nil
	}
	v_list, ok := v.([]interface{})
	if ok {
		frag, err := serializeList(v_list)
		if err != nil {
			return nil, err
		}
		buf := new(bytes.Buffer)
		buf.WriteByte('[')
		buf.Write(frag)
		buf.WriteByte(']')
		return buf.Bytes(), nil
	}
	v_str, ok := v.(string)
	if ok {
		return serializeString(v_str), nil
	}
	v_float64, ok := v.(float64)
	if ok {
		return []byte(strconv.FormatInt(int64(v_float64), 10)), nil
	}
	return nil, &SerializeError{"", fmt.Sprintf("unknown type [%T]", v)}
}

func serializeDict(d map[string]interface{}, in map[string]bool, ex map[string]bool) ([]byte, *SerializeError) {
	buf := new(bytes.Buffer)
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if (in != nil && !in[k]) || (ex != nil && ex[k]) {
			continue
		}
		v := d[k]
		frag, err := serializeValue(v)
		if err != nil {
			return nil, &SerializeError{"." + k + err.position, err.msg}
		}
		if buf.Len() > 0 {
			buf.WriteByte('.')
		}
		buf.Write(serializeString(k))
		buf.WriteByte('.')
		buf.Write(frag)
	}
	return buf.Bytes(), nil
}

func SerializeMap(d map[string]interface{}, i map[string]bool, e map[string]bool) ([]byte, error) {
	value, err := serializeDict(d, i, e)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func SerializeJSON(s []byte, in map[string]bool, exclude map[string]bool) ([]byte, error) {
	var params map[string]interface{}
	if err := json.Unmarshal(s, &params); err != nil {
		return nil, err
	}
	data, err := serializeDict(params, in, exclude)
	if err != nil {
		return nil, err
	}
	return data, nil
}
