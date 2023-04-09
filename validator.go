package validate

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var ErrNotStruct = errors.New("wrong argument given, should be a struct")
var ErrInvalidValidatorSyntax = errors.New("invalid validator syntax")
var ErrValidateForUnexportedFields = errors.New("validation for unexported field is not allowed")

type ValidationError struct {
	Err error
}

type ValidationErrors []ValidationError

type validator struct {
	assertInt func(val int, keyVal string) (bool, error)
	assertStr func(val string, keyVal string) (bool, error)
}

var validators = map[string]validator{
	"len": {
		assertInt: func(val int, keyVal string) (bool, error) {
			return true, nil
		},
		assertStr: func(val, keyVal string) (bool, error) {
			trueLen, err := strconv.Atoi(keyVal)
			if err != nil {
				return false, ErrInvalidValidatorSyntax
			}
			return len(val) == trueLen, nil
		},
	},
	"in": {
		assertInt: func(val int, keyVal string) (bool, error) {
			set := strings.Split(keyVal, ",")
			for _, elem := range set {
				elemInt, err := strconv.Atoi(elem)
				if err != nil {
					return false, ErrInvalidValidatorSyntax
				}
				if val == elemInt {
					return true, nil
				}
			}
			return false, nil
		},
		assertStr: func(val, keyVal string) (bool, error) {
			set := strings.Split(keyVal, ",")
			for _, elem := range set {
				if val == elem {
					return true, nil
				}
			}
			return false, nil
		},
	},
	"min": {
		assertInt: func(val int, keyVal string) (bool, error) {
			min, err := strconv.Atoi(keyVal)
			if err != nil {
				return false, ErrInvalidValidatorSyntax
			}
			return val >= min, nil
		},
		assertStr: func(val, keyVal string) (bool, error) {
			min, err := strconv.Atoi(keyVal)
			if err != nil {
				return false, ErrInvalidValidatorSyntax
			}
			return len(val) >= min, nil
		},
	},
	"max": {
		assertInt: func(val int, keyVal string) (bool, error) {
			max, err := strconv.Atoi(keyVal)
			if err != nil {
				return false, ErrInvalidValidatorSyntax
			}
			return val <= max, nil
		},
		assertStr: func(val, keyVal string) (bool, error) {
			max, err := strconv.Atoi(keyVal)
			if err != nil {
				return false, ErrInvalidValidatorSyntax
			}
			return len(val) <= max, nil
		},
	},
}

var TagRegexp = regexp.MustCompile(`^(?:([a-z]+):([[:alnum:]:,-]*))(?:;([a-z]+):([[:alnum:]:,-]*))*?$`)

func (v ValidationErrors) Error() (res string) {
	for _, err := range v {
		res += err.Err.Error()
	}
	return
}

func (v *validator) Validate(tagVal string, vFieldVal any) (res bool, err error) {
	if len(tagVal) == 0 {
		return false, nil
	}
	totalOk := false
	if valInt, ok := vFieldVal.(int); ok {
		totalOk = true
		res, err = v.assertInt(valInt, tagVal)
		if err != nil || !res {
			return
		}
	}
	if valStr, ok := vFieldVal.(string); ok {
		totalOk = true
		res, err = v.assertStr(valStr, tagVal)
		if err != nil || !res {
			return
		}
	}
	if !totalOk {
		err = fmt.Errorf("unsupported type %T", vFieldVal)
		return
	}
	return true, nil
}

func Validate(v any) error {
	vVal := reflect.ValueOf(v)
	if vVal.Kind() != reflect.Struct {
		return ErrNotStruct
	}
	valErrs, err := validateImpl(vVal, make([]string, 0), "")
	if err != nil {
		return ValidationErrors{ValidationError{err}}
	}
	if len(valErrs) == 0 {
		return nil
	}
	return valErrs
}

func validateImpl(vVal reflect.Value, vTags []string, callstack string) (valErrs ValidationErrors, err error) {
	if vVal.Type().Kind() == reflect.Array || vVal.Type().Kind() == reflect.Slice {
		for i := 0; i < vVal.Len(); i++ {
			newValErrs, err := validateImpl(vVal.Index(i), vTags, callstack+fmt.Sprintf("[%d]", i))
			if err != nil {
				return nil, err
			}
			valErrs = append(valErrs, newValErrs...)
		}
	} else if vVal.Type().Kind() == reflect.Struct {
		for i := 0; i < vVal.Type().NumField(); i++ {
			field := vVal.Type().Field(i)
			tag, tagOk := field.Tag.Lookup("validate")
			if tagOk && !field.IsExported() {
				return nil, ErrValidateForUnexportedFields
			}
			newValErrs, err := validateImpl(vVal.Field(i), append(vTags, tag), callstack+"."+field.Name)
			if err != nil {
				return nil, err
			}
			valErrs = append(valErrs, newValErrs...)
		}
	} else {
		for _, tag := range vTags {
			if len(tag) == 0 {
				continue
			}
			matches := TagRegexp.FindStringSubmatch(tag)
			if matches == nil || len(matches) < 3 {
				return nil, ErrInvalidValidatorSyntax
			}
			for i := 1; i < len(matches); i += 2 {
				if len(matches[i]) == 0 {
					break
				}
				tagKey, tagVal := matches[i], matches[i+1]
				validator, exists := validators[tagKey]
				if !exists {
					return nil, fmt.Errorf("%v: unsupported tag %q", ErrInvalidValidatorSyntax, tagKey)
				}
				var res bool
				res, err = validator.Validate(tagVal, vVal.Interface())
				if err != nil {
					return
				}
				if !res {
					valErrs = append(valErrs, ValidationError{fmt.Errorf("%s: validation failed for %q tag", callstack, tagKey)})
				}
			}
		}
	}
	return
}
