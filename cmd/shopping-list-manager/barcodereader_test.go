package main

import (
	"testing"

	"github.com/function61/gokit/app/evdev"
	"github.com/function61/gokit/testing/assert"
)

func TestKeyCodesToText(t *testing.T) {
	assert.Equal(t, keyCodesToText([]evdev.KeyOrButton{}), "")

	assert.Equal(t, keyCodesToText([]evdev.KeyOrButton{
		evdev.KeyH,
		evdev.KeyT,
		evdev.KeyT,
		evdev.KeyP,
		evdev.KeyS,
		evdev.KeyLEFTSHIFT,
		evdev.KeySEMICOLON,
		evdev.KeySLASH,
		evdev.KeySLASH,
		evdev.KeyX,
		evdev.KeyS,
		evdev.KeyDOT,
		evdev.KeyF,
		evdev.KeyI,
		evdev.KeySLASH,
		evdev.Key0,
		evdev.KeySLASH,
		evdev.KeyLEFTSHIFT,
		evdev.KeyU,
		evdev.KeyLEFTSHIFT,
		evdev.KeyJ,
		evdev.KeyLEFTSHIFT,
		evdev.KeyN,
		evdev.KeyY,
		evdev.KeyLEFTSHIFT,
		evdev.KeyJ,
		evdev.KeyM,
		evdev.KeyK,
	}), "https://xs.fi/0/UJNyJmk")

	assert.Equal(t, keyCodesToText([]evdev.KeyOrButton{
		evdev.KeyF,
		evdev.KeyO,
		evdev.KeyO,
		evdev.KeyMINUS,
		evdev.KeyLEFTSHIFT,
		evdev.KeyMINUS,
	}), "foo-_")
}
