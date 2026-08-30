package deptest

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestDecimal(t *testing.T) {
	v1, err := decimal.NewFromString("0.05")
	t.Log(err)
	t.Log(v1)
	v2, _ := decimal.NewFromString("1000000")
	t.Log(v2)

	r := v1.Div(v2)
	t.Log(r)

	t.Log(0.05 / 1000000)
}
