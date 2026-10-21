package media

import "testing"

func TestParameterizedIIDs(t *testing.T) {
	for signature, want := range map[string]string{
		"pinterface({913337e9-11a1-4345-a3a2-4e7f956e222d};string)": "{98b9acc1-4b56-532e-ac73-03d5291cca90}",
		"pinterface({faa585ea-6214-4217-afda-7f46de5869b3};string)": "{e2fcc7c1-3bfc-5a0b-b2b0-72e769d1cb7e}",
	} {
		if got := parameterizedIID(signature).String(); got != want {
			t.Errorf("%s: got %s, want %s", signature, got, want)
		}
	}

	if got := iidButtonPressedHandler.String(); got != "{0557e996-7b23-5bae-aa81-ea0d671143a4}" {
		t.Errorf("the button handler's interface is %s", got)
	}
}

func TestButtonsBecomeCommands(t *testing.T) {
	for button, want := range map[transportButton]Command{
		buttonPlay:        Play,
		buttonPause:       Pause,
		buttonStop:        Pause,
		buttonNext:        Next,
		buttonPrevious:    Previous,
		buttonFastForward: SkipForward,
		buttonRewind:      SkipBackward,
	} {
		if got, ok := commandForButton(button); !ok || got != want {
			t.Errorf("button %d gave %v, want %v", button, got, want)
		}
	}

	if _, ok := commandForButton(3); ok {
		t.Error("the record button should be ignored")
	}
}
