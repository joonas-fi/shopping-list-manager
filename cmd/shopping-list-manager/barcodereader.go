package main

// Reads barcodes from a barcode scanner using evdev (in grabbed mode - meaning no other programs sees the "keyboard input")

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/function61/gokit/app/evdev"
	"github.com/function61/gokit/sync/syncutil"
)

func readBarcodes(ctx context.Context, barcodeReader *evdev.Device, beep chan<- string, logger *log.Logger) error {
	scanInputStopped := syncutil.Async(func() error { return barcodeReader.ScanInputGrabbed(ctx) })

	// only "committed" once we get enter keyrelease
	keysEntered := []evdev.KeyOrButton{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-scanInputStopped:
			return err
		case input, ok := <-barcodeReader.Input:
			if !ok {
				return errors.New("input closed unexpectedly")
			}

			isKeyRelease := input.Type == evdev.EvKey && input.Value == evdev.KeyRelease
			// per each keypress and key release we get EV_MSC and EV_SYN, so they seem rather useless.
			// only care about key releases
			if !isKeyRelease {
				continue
			}

			keyCode := evdev.KeyOrButton(input.Code)

			// barcode readers act like keyboard that types `<barcode>ENTER`
			barcodeEnteredFully := keyCode == evdev.KeyENTER

			if barcodeEnteredFully {
				select {
				case beep <- keyCodesToText(keysEntered):
				// happy
				default:
					logger.Printf("beep channel overflowed. dropped [%s]", keysEntered)
				}

				keysEntered = []evdev.KeyOrButton{}
			} else {
				keysEntered = append(keysEntered, keyCode)
			}
		}
	}
}

// turns something like `["LEFTSHIFT", "F", "O", "O"]` into `"Foo"`.
// this is the reason we need to process the input as a whole because each item could be influenced by previous (like shift being on)
//
// FIXME: this is really crude purpose-built solution for just one behaviour - this does not generalize.
func keyCodesToText(keys []evdev.KeyOrButton) string {
	text := ""

	for i := range len(keys) {
		if !isPrintableKey(keys[i]) {
			continue
		}

		shiftPressed := i >= 1 && isShift(keys[i-1])

		char := func() string {
			// for some cases the `.String()` func doesn't return the "char" but instead textual representation like `"SLASH"`
			//nolint:exhaustive // doesn't need to be
			switch keys[i] {
			case evdev.KeySEMICOLON:
				return ":"
			case evdev.KeySLASH:
				return "/"
			case evdev.KeyDOT:
				return "."
			case evdev.KeyMINUS:
				if shiftPressed {
					return "_"
				} else {
					return "-"
				}
			default:
				// for 0-9, A-Z etc this works
				return keys[i].String()
			}
		}()

		// the key for "F" is represented in uppercase on the keyboard, but whether we get lowercase or uppercase
		// depends on caps lock or press of shift.
		if !shiftPressed {
			char = strings.ToLower(char)
		}

		text += char
	}

	return text
}

func isShift(keyCode evdev.KeyOrButton) bool {
	return keyCode == evdev.KeyLEFTSHIFT || keyCode == evdev.KeyRIGHTSHIFT
}

func isPrintableKey(keyCode evdev.KeyOrButton) bool {
	/*
	   observation: if system (not just this barcode reader input device) has caps lock enabled, and
	   barcode reader sends 1234 we seem to get "<capslock>1234<enter>" so the capslock seems to be synthesized

	   ChatGPT answer:

	   Q: Why does Linux evdev device emit capslock event for each scanned barcode if there is capslock turned on in the system?

	   A: The behavior you're describing with the evdev input system in Linux—where a caps lock event is emitted with each scanned barcode if caps lock is turned on—can occur due to a few key reasons tied to how the input system and barcode scanners interact:

	   1. Caps Lock Behavior with Barcode Scanners
	   Most barcode scanners emulate a keyboard to send scanned data. When caps lock is enabled on the system, the scanner, acting as a keyboard, adheres to the keyboard state. This can cause:

	   Uppercase/Lowercase Character Issue: Scanners often send keycodes directly for each character, and with caps lock on, the characters could be unexpectedly uppercase or lowercase, depending on the scanner's configuration.
	   Explicit Keycode Emission: Some scanners may explicitly emit a caps lock state before or after the scan to ensure consistent character output. This may be interpreted by evdev as caps lock events.
	*/
	return keyCode != evdev.KeyCAPSLOCK && !isShift(keyCode)
}
