package cast

import "fmt"

// AnyToInt converts common numeric dynamic values to int.
func AnyToInt(raw any) (int, error) {
	switch typed := raw.(type) {
	case int:
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case uint:
		return int(typed), nil
	case uint8:
		return int(typed), nil
	case uint16:
		return int(typed), nil
	case uint32:
		return int(typed), nil
	case uint64:
		return int(typed), nil
	case float32:
		return int(typed), nil
	case float64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("unsupported number type: %T", raw)
	}
}

// AnyToIntSlice converts common dynamic slice values to []int.
func AnyToIntSlice(raw any) ([]int, error) {
	switch typed := raw.(type) {
	case []int:
		return typed, nil
	case []int32:
		out := make([]int, 0, len(typed))
		for _, v := range typed {
			out = append(out, int(v))
		}
		return out, nil
	case []int64:
		out := make([]int, 0, len(typed))
		for _, v := range typed {
			out = append(out, int(v))
		}
		return out, nil
	case []float64:
		out := make([]int, 0, len(typed))
		for _, v := range typed {
			out = append(out, int(v))
		}
		return out, nil
	case []any:
		out := make([]int, 0, len(typed))
		for idx, item := range typed {
			v, err := AnyToInt(item)
			if err != nil {
				return nil, fmt.Errorf("cast item at index %d: %w", idx, err)
			}
			out = append(out, v)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported slice type: %T", raw)
	}
}
