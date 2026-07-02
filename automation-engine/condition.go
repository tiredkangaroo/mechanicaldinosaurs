package main

import (
	"fmt"
	"regexp"
	"strings"
)

// let's keep conditions simple for now
type Condition struct {
	Variable string   `json:"variable"` // variable to check, e.g. "machines_info.pineapple.cpu_temp"
	Operator Operator `json:"op"`       // operator to use for comparison
	Value    any      `json:"value"`    // value to compare against, e.g. 80, only supports string, numbers, bool rn
	Not      bool     `json:"not"`      // negation of the condition
}

func (c *Condition) String() string {
	v := fmt.Sprintf("%s %s %v", c.Variable, c.Operator, c.Value)
	if c.Not {
		return fmt.Sprintf("NOT (%s)", v)
	}
	return v
}

func (c *Condition) Evaluate(variables Context) (bool, error) {
	result, err := c.evaluateWithoutNegation(variables)
	if err != nil {
		return false, err
	}

	if c.Not {
		return !result, nil
	}
	return result, nil
}

func (c *Condition) evaluateWithoutNegation(variables Context) (bool, error) {
	value, ok := variables.Get(c.Variable)
	if !ok {
		return false, fmt.Errorf("variable not found: %s", c.Variable)
	}

	valueNum, errValueNum := num(value)
	cValueNum, errCValueNum := num(c.Value)

	switch c.Operator {
	case OperatorGreaterThan, OperatorLessThan, OperatorGreaterThanOrEqual, OperatorLessThanOrEqual:
		if errValueNum != nil || errCValueNum != nil {
			return false, fmt.Errorf("operator %v requires numeric values, but got: %v and %v", c.Operator, value, c.Value)
		}
	}

	valueStr, okValueStr := value.(string)
	cValueStr, okCValueStr := c.Value.(string)

	switch c.Operator {
	case OperatorContains, OperatorStartsWith, OperatorEndsWith, OperatorRegexMatch:
		if !okValueStr || !okCValueStr {
			return false, fmt.Errorf("operator %v requires string values, but got: %v and %v", c.Operator, value, c.Value)
		}
	}

	switch c.Operator {
	case OperatorEquals:
		if errValueNum == nil && errCValueNum == nil {
			return valueNum == cValueNum, nil
		}
		valueBool, okValueBool := value.(bool)
		cValueBool, okCValueBool := c.Value.(bool)
		if okValueBool && okCValueBool {
			return valueBool == cValueBool, nil
		}
		return false, fmt.Errorf("operator equals requires comparable types, but got: %v and %v", value, c.Value)
	case OperatorGreaterThan:
		return valueNum > cValueNum, nil
	case OperatorLessThan:
		return valueNum < cValueNum, nil
	case OperatorGreaterThanOrEqual:
		return valueNum >= cValueNum, nil
	case OperatorLessThanOrEqual:
		return valueNum <= cValueNum, nil
	case OperatorContains:
		return strings.Contains(valueStr, cValueStr), nil
	case OperatorStartsWith:
		return strings.HasPrefix(valueStr, cValueStr), nil
	case OperatorEndsWith:
		return strings.HasSuffix(valueStr, cValueStr), nil
	case OperatorRegexMatch:
		matched, err := regexp.MatchString(cValueStr, valueStr)
		if err != nil {
			return false, fmt.Errorf("invalid regex pattern: %v", err)
		}
		return matched, nil
	default:
		return false, fmt.Errorf("unsupported operator: %v", c.Operator)
	}
}

type Operator string

const (
	OperatorEquals             Operator = "equals"
	OperatorGreaterThan        Operator = "greater than"
	OperatorLessThan           Operator = "less than"
	OperatorGreaterThanOrEqual Operator = "greater than or equal"
	OperatorLessThanOrEqual    Operator = "less than or equal"
	OperatorContains           Operator = "contains"
	OperatorStartsWith         Operator = "starts with"
	OperatorEndsWith           Operator = "ends with"
	OperatorRegexMatch         Operator = "regex match"
)

func num(a any) (float64, error) {
	var aNum float64
	switch a := a.(type) {
	case int:
		aNum = float64(a)
	case int8:
		aNum = float64(a)
	case int16:
		aNum = float64(a)
	case int32:
		aNum = float64(a)
	case int64:
		aNum = float64(a)
	case uint:
		aNum = float64(a)
	case uint8:
		aNum = float64(a)
	case uint16:
		aNum = float64(a)
	case uint32:
		aNum = float64(a)
	case uint64:
		aNum = float64(a)
	case float32:
		aNum = float64(a)
	case float64:
		aNum = a
	default:
		return 0, fmt.Errorf("unsupported type for number conversion: %T", a)
	}
	return aNum, nil
}
