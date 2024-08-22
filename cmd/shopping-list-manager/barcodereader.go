package main

// Reads barcodes from a barcode scanner using evdev (in grabbed mode - meaning no other programs sees the "keyboard input")

import (
	"context"
	"errors"
	"log"

	"github.com/function61/gokit/app/evdev"
	"github.com/function61/gokit/sync/syncutil"
)

func readBarcodes(ctx context.Context, barcodeReader *evdev.Device, beep chan<- string, logger *log.Logger) error {
	scanInputStopped := syncutil.Async(func() error { return barcodeReader.ScanInputGrabbed(ctx) })

	// only "committed" once we get enter keyrelease
	codeBuffer := ""

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
				case beep <- codeBuffer:
				// happy
				default:
					logger.Printf("beep channel overflowed. dropped [%s]", codeBuffer)
				}

				codeBuffer = ""
			} else {
				// taking advantage of the fact that barcode symbology (0-9a-z and so on) the strings
				// tend to have length of 1 and represent exactly the symbol we're interested in
				codeBuffer += keyCode.String()
			}
		}
	}
}
