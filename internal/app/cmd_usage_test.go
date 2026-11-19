package app

import (
	"reflect"
	"testing"
)

// TestSpendBefore pins how finely spendBefore tells resets apart, with a
// fixed now: the parity test (TestUsageSuggestsSoonestReset) runs on the
// wall clock, so where an hour or a minute boundary falls is up to it.
func TestSpendBefore(t *testing.T) {
	const now = int64(1_800_000_000) + 1234 // off any hour or minute boundary
	const h, m = int64(3600), int64(60)
	// 5-hour resets are told apart by the clock minute ("13:40")
	at := (now/m + 61) * m // a whole minute about an hour on
	cases := []struct {
		name string
		in   []useFirst
		want []string
	}{
		{"7-day: same whole hours left tie, the 5-hour reset decides", []useFirst{
			{name: "a", r7: now + 5*h + 1*m, r5: now + 4*h},
			{name: "b", r7: now + 5*h + 59*m, r5: now + 1*h},
		}, []string{"b", "a"}},
		{"7-day: an hour more left wins, whatever the 5-hour reset", []useFirst{
			{name: "a", r7: now + 6*h + 1*m, r5: now + 1*h},
			{name: "b", r7: now + 5*h + 59*m, r5: now + 4*h},
		}, []string{"b", "a"}},
		{"7-day: by hours, not days", []useFirst{
			{name: "a", r7: now + 2*24*h + 20*h, r5: now + 1*h},
			{name: "b", r7: now + 2*24*h + 3*h, r5: now + 4*h},
		}, []string{"b", "a"}},
		{"5-hour: same minute ties, most room left decides", []useFirst{
			{name: "a", r7: now + 5*h, r5: at + 50, used: 0.1},
			{name: "b", r7: now + 5*h, r5: at + 5, used: 0.6},
		}, []string{"a", "b"}},
		{"5-hour: a minute sooner wins, whatever the use", []useFirst{
			{name: "a", r7: now + 5*h, r5: at + 1*m, used: 0.1},
			{name: "b", r7: now + 5*h, r5: at + 59, used: 0.6},
		}, []string{"b", "a"}},
		{"all alike: the name", []useFirst{
			{name: "z", r7: now + 5*h, r5: now + h, used: 0.2},
			{name: "a", r7: now + 5*h, r5: now + h, used: 0.2},
		}, []string{"a", "z"}},
		{"a reset never told: last", []useFirst{
			{name: "a", r7: 0, r5: now + h},
			{name: "b", r7: now + 9*24*h, r5: now + 4*h},
		}, []string{"b", "a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spendBefore(c.in, now)
			var got []string
			for _, u := range c.in {
				got = append(got, u.name)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("order %v, want %v", got, c.want)
			}
		})
	}
}
