// cmd/oioni/ui/scene_helpers_test.go
package ui

import (
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// fakeDisplay satisfies gui.Display with no-op methods for tests.
type fakeDisplay struct{}

func (fakeDisplay) Init(_ epd.Mode) error         { return nil }
func (fakeDisplay) DisplayBase(_ []byte) error    { return nil }
func (fakeDisplay) DisplayPartial(_ []byte) error { return nil }
func (fakeDisplay) DisplayFast(_ []byte) error    { return nil }
func (fakeDisplay) DisplayRegenerate() error      { return nil }
func (fakeDisplay) Sleep() error                  { return nil }
func (fakeDisplay) Close() error                  { return nil }

func TestCategoryScene_SingleTopLevelWidget(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	for _, tc := range []struct {
		name string
		fn   func(*gui.Navigator) *gui.Scene
	}{
		{"Config", func(nav *gui.Navigator) *gui.Scene { return NewConfigScene(nav, nil, nil) }},
		{"System", NewSystemScene},
		{"Attack", NewAttackScene},
		{"DFIR", NewDFIRScene},
		{"Info", NewInfoScene},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.fn(nav)
			if len(s.Widgets) != 1 {
				t.Fatalf("expected 1 top-level widget, got %d (sidebar must not be listed separately)", len(s.Widgets))
			}
		})
	}
}

func TestCategoryScene_Title(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	cases := []struct {
		fn    func(*gui.Navigator) *gui.Scene
		title string
	}{
		{func(nav *gui.Navigator) *gui.Scene { return NewConfigScene(nav, nil, nil) }, "Config"},
		{NewSystemScene, "System"},
		{NewAttackScene, "Attack"},
		{NewDFIRScene, "DFIR"},
		{NewInfoScene, "Info"},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			s := tc.fn(nav)
			if s.Title != tc.title {
				t.Errorf("expected title %q, got %q", tc.title, s.Title)
			}
		})
	}
}

func TestCategoryScene_ExtraSidebarBtn(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	called := false
	_ = newCategoryScene(nav, "Test", gui.NewLabel("x"),
		withExtraSidebarBtn(gui.Icon{}, func() { called = true }),
	)
	_ = called
}
