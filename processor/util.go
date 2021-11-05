package processor

func checkIfStringType(params []interface{}) (string, error) {
	if len(params) < 1 {
		return "", ErrMissedRequiredParams
	}
	if s, ok := params[0].(string); ok {
		return s, nil
	} else {
		return "", ErrIncorrectParamType
	}
}

func checkIfIntType(params []interface{}) (int, error) {
	if len(params) < 1 {
		return 0, ErrMissedRequiredParams
	}
	if s, ok := params[0].(int); ok {
		return s, nil
	} else {
		return 0, ErrIncorrectParamType
	}
}

func checkParserToInt(param interface{}) (int, error) {
	switch d := param.(type) {
	case int64:
		return int(d), nil
	case int:
		return d, nil
	case float64:
		return int(d), nil
	default:
		return 0, ErrMissedRequiredParams
	}
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
