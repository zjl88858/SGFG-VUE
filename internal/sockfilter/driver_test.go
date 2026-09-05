package sockfilter

import (
	"errors"
	"reflect"
	"testing"
)

type fakeDriver struct {
	steps []string
	fail  string
}

func (d *fakeDriver) step(s string) error {
	d.steps = append(d.steps, s)
	if d.fail == s {
		return errors.New(s)
	}
	return nil
}
func (d *fakeDriver) Install() error { return d.step("install") }
func (d *fakeDriver) Start() error   { return d.step("start") }
func (d *fakeDriver) Stop() error    { return d.step("stop") }
func (d *fakeDriver) Remove() error  { return d.step("remove") }
func (d *fakeDriver) Close()         { d.steps = append(d.steps, "close") }

func TestDriverLifecycle(t *testing.T) {
	for _, tc := range []struct {
		fail string
		want []string
	}{
		{"", []string{"install", "start", "stop", "remove", "close"}},
		{"install", []string{"install", "close"}},
		{"start", []string{"install", "start", "stop", "remove", "close"}},
		{"stop", []string{"install", "start", "stop", "close"}},
		{"remove", []string{"install", "start", "stop", "remove", "close"}},
	} {
		t.Run(tc.fail, func(t *testing.T) {
			b := &fakeDriver{fail: tc.fail}
			l, err := startManagedDriver(b)
			if l != nil {
				err = l.Close()
				again := l.Close()
				if !reflect.DeepEqual(err, again) {
					t.Fatal("close not idempotent")
				}
			}
			if (err != nil) != (tc.fail != "") {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(b.steps, tc.want) {
				t.Fatalf("got %v want %v", b.steps, tc.want)
			}
		})
	}
}
