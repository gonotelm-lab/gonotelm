package deptest

import (
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/expr-lang/expr"
)

type ExprEnv struct {
	Model string    `expr:"model"`
	Ts    time.Time `expr:"ts"`
}

func TestExprConditional(t *testing.T) {
	program, err := expr.Compile(
		`model == 'a' ? {price: 100, name: 'yes'} :
		 model == 'b' ? {price: 120, name: 'ok'} :
		 {price: 200, name: 'tim'}`,
		expr.Env(ExprEnv{}),
	)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	for _, model := range []string{"a", "b", "c"} {
		output, err := expr.Run(program, ExprEnv{Model: model})
		if err != nil {
			t.Fatalf("model=%s: %v", model, err)
		}
		t.Logf("model=%s -> %v", model, output)
	}
}

func TestExprTimePricing(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	program, err := expr.Compile(
		`let t = ts.In(timezone("Asia/Shanghai"));
		 let hour = t.Hour();
		 let workday = int(t.Weekday()) >= 1 && int(t.Weekday()) <= 5;
		 let peak = workday && ((hour >= 9 && hour < 12) || (hour >= 14 && hour < 18));
		 let base = model == 'a' ? 100 : model == 'b' ? 120 : 200;
		 let name = model == 'a' ? 'yes' : model == 'b' ? 'ok' : 'tim';
		 {price: peak ? base : base / 2, name: name, peak: peak}`,
		expr.Env(ExprEnv{}),
	)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	cases := []struct {
		model string
		ts    time.Time
	}{
		{"a", time.Date(2026, time.August, 24, 10, 0, 0, 0, shanghai)}, // Mon 10:00 peak
		{"a", time.Date(2026, time.August, 24, 22, 0, 0, 0, shanghai)}, // Mon 22:00 off-peak
		{"b", time.Date(2026, time.August, 26, 15, 0, 0, 0, shanghai)}, // Wed 15:00 peak
		{"b", time.Date(2026, time.August, 29, 10, 0, 0, 0, shanghai)}, // Sat 10:00 weekend
		{"c", time.Date(2026, time.August, 28, 18, 0, 0, 0, shanghai)}, // Fri 18:00 boundary
		{"c", time.Date(2026, time.August, 25, 13, 0, 0, 0, shanghai)}, // Tue 13:00 lunch gap
		{"a", time.Date(2026, time.August, 24, 9, 0, 0, 0, shanghai)},  // Mon 09:00 peak start
		{"a", time.Date(2026, time.August, 24, 12, 0, 0, 0, shanghai)}, // Mon 12:00 peak end
	}

	for _, c := range cases {
		output, err := expr.Run(program, ExprEnv{Model: c.model, Ts: c.ts})
		if err != nil {
			t.Fatalf("model=%s ts=%s: %v", c.model, c.ts, err)
		}
		t.Logf("model=%s ts=%s -> %v", c.model, c.ts.Format("2006-01-02 15:04 Mon"), output)
		t.Logf("%T", output)
	}
}
