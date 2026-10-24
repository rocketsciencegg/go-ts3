package ts3

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
)

var (
	// encoder performs white space and special character encoding
	// as required by the ServerQuery protocol.
	encoder = strings.NewReplacer(
		`\`, `\\`,
		`/`, `\/`,
		` `, `\s`,
		`|`, `\p`,
		"\a", `\a`,
		"\b", `\b`,
		"\f", `\f`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
		"\v", `\v`,
	)

	// decoder performs white space and special character decoding
	// as required by the ServerQuery protocol.
	decoder = strings.NewReplacer(
		`\\`, "\\",
		`\/`, "/",
		`\s`, " ",
		`\p`, "|",
		`\a`, "\a",
		`\b`, "\b",
		`\f`, "\f",
		`\n`, "\n",
		`\r`, "\r",
		`\t`, "\t",
		`\v`, "\v",
	)
)

// Decode returns a decoded version of str.
func Decode(str string) string {
	return decoder.Replace(str)
}

// DecodeResponse decodes a response into a struct.
func DecodeResponse(lines []string, v interface{}) error {
	if len(lines) > 1 {
		return NewInvalidResponseError("too many lines", lines)
	} else if len(lines) == 0 {
		return NewInvalidResponseError("no lines", lines)
	}

	input := make(map[string]interface{})
	value := reflect.ValueOf(v)
	var slice reflect.Value
	var elemType reflect.Type
	if value.Kind() == reflect.Pointer {
		slice = value.Elem()
		if slice.Kind() == reflect.Slice {
			elemType = slice.Type().Elem()
		}
	}

	for _, part := range strings.Split(lines[0], "|") {
		for _, val := range strings.Split(part, " ") {
			parts := strings.SplitN(val, "=", 2)
			key := Decode(parts[0])
			if len(parts) == 2 {
				v := Decode(parts[1])
				if i, err := strconv.Atoi(v); err != nil {
					// Only support comma separated lists
					// by keyname to avoid incorrect decoding.
					if key == "client_servergroups" {
						parts := strings.Split(v, ",")
						serverGroups := make([]int, len(parts))
						for i, s := range parts {
							group, err := strconv.Atoi(s)
							if err != nil {
								return fmt.Errorf("decode server group: %w", err)
							}
							serverGroups[i] = group
						}
						input[key] = serverGroups
					} else {
						input[key] = v
					}
				} else {
					input[key] = i
				}
			} else {
				input[key] = ""
			}
		}

		if elemType != nil {
			// Expecting a slice
			if err := decodeSlice(elemType, slice, input); err != nil {
				return err
			}

			// Reset the input map
			input = make(map[string]interface{})
		}
	}

	if elemType != nil {
		// Expecting a slice, already decoded
		return nil
	}

	return decodeMap(input, v)
}

// decodeMap decodes input into r.
func decodeMap(d map[string]interface{}, r interface{}) error {
	cfg := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		TagName:          "ms",
		Result:           r,
		DecodeHook:       timeHookFunc,
	}
	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return fmt.Errorf("decode map: new decoder %w", err)
	}
	if err := dec.Decode(d); err != nil {
		return fmt.Errorf("decode map: decode: %w", err)
	}

	return nil
}

// decodeSlice decodes input into slice.
func decodeSlice(elemType reflect.Type, slice reflect.Value, input map[string]interface{}) error {
	var v reflect.Value
	if elemType.Kind() == reflect.Pointer {
		v = reflect.New(elemType.Elem())
	} else {
		v = reflect.New(elemType)
	}

	if !v.CanInterface() {
		return fmt.Errorf("can't interface %#v", v)
	}

	// The mapstructure's decoder doesn't support squashing
	// for embedded pointers to structs (the type is lost when
	// using reflection for nil values). We need to add pointers
	// to empty structs within the interface to get around this.
	if client, ok := v.Interface().(*OnlineClient); ok {
		ext := &OnlineClientExt{
			OnlineClientGroups: &OnlineClientGroups{},
			OnlineClientInfo:   &OnlineClientInfo{},
			OnlineClientTimes:  &OnlineClientTimes{},
			OnlineClientVoice:  &OnlineClientVoice{},
		}
		client.OnlineClientExt = ext
	}

	if err := decodeMap(input, v.Interface()); err != nil {
		return err
	}

	// nil out empty structs
	if client, ok := v.Interface().(*OnlineClient); ok {
		ext := client.OnlineClientExt
		emptyExt := OnlineClientExt{}
		emptyExtGroups := OnlineClientGroups{}
		emptyExtInfo := OnlineClientInfo{}
		emptyExtTimes := OnlineClientTimes{}
		emptyExtVoice := OnlineClientVoice{}

		if *ext.OnlineClientGroups == emptyExtGroups {
			client.OnlineClientGroups = nil
		}

		if *ext.OnlineClientInfo == emptyExtInfo {
			client.OnlineClientInfo = nil
		}

		if *ext.OnlineClientTimes == emptyExtTimes {
			client.OnlineClientTimes = nil
		}

		if *ext.OnlineClientVoice == emptyExtVoice {
			client.OnlineClientVoice = nil
		}

		if *ext == emptyExt {
			v.Interface().(*OnlineClient).OnlineClientExt = nil
		}
	}

	if elemType.Kind() == reflect.Struct {
		v = v.Elem()
	}
	slice.Set(reflect.Append(slice, v))

	return nil
}

var timeType = reflect.TypeOf(time.Time{})

// timeHookFunc supports decoding to time.
func timeHookFunc(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
	// Decode time.Time
	if to == timeType {
		var timeInt int64

		switch from.Kind() {
		case reflect.Int:
			timeInt = int64(data.(int))
		case reflect.String:
			var err error
			timeInt, err = strconv.ParseInt(data.(string), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid time %q: %w", data, err)
			}
		}

		if timeInt > 0 {
			return time.Unix(timeInt, 0), nil
		}
	}

	return data, nil
}
